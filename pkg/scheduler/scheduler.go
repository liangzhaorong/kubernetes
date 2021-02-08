/*
Copyright 2014 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package scheduler

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	schedulerapi "k8s.io/kubernetes/pkg/scheduler/apis/config"
	"k8s.io/kubernetes/pkg/scheduler/apis/config/scheme"
	"k8s.io/kubernetes/pkg/scheduler/core"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	frameworkplugins "k8s.io/kubernetes/pkg/scheduler/framework/plugins"
	frameworkruntime "k8s.io/kubernetes/pkg/scheduler/framework/runtime"
	internalcache "k8s.io/kubernetes/pkg/scheduler/internal/cache"
	"k8s.io/kubernetes/pkg/scheduler/internal/parallelize"
	internalqueue "k8s.io/kubernetes/pkg/scheduler/internal/queue"
	"k8s.io/kubernetes/pkg/scheduler/metrics"
	"k8s.io/kubernetes/pkg/scheduler/profile"
	"k8s.io/kubernetes/pkg/scheduler/util"
)

const (
	// SchedulerError is the reason recorded for events when an error occurs during scheduling a pod.
	SchedulerError = "SchedulerError"
	// Percentage of plugin metrics to be sampled.
	pluginMetricsSamplePercent = 10
)

// Scheduler watches for new unscheduled pods. It attempts to find
// nodes that they fit on and writes bindings back to the api server.
//
// Scheduler 监控新的未调度的（unscheduled）的 pods. 它尝试查找合适的节点, 并将 binding 发送给 kube-apiserver.
type Scheduler struct {
	// It is expected that changes made via SchedulerCache will be observed
	// by NodeLister and Algorithm.
	SchedulerCache internalcache.Cache

	// 指向 genericScheduler 对象
	Algorithm core.ScheduleAlgorithm

	// NextPod should be a function that blocks until the next pod
	// is available. We don't use a channel for this, because scheduling
	// a pod may take some amount of time and we don't want pods to get
	// stale while they sit in a channel.
	NextPod func() *framework.QueuedPodInfo

	// Error is called if there is an error. It is passed the pod in
	// question, and the error
	Error func(*framework.QueuedPodInfo, error)

	// Close this to shut down the scheduler.
	StopEverything <-chan struct{}

	// SchedulingQueue holds pods to be scheduled
	// 调度队列, 存储了待调度 Pod 资源对象. 调度队列的实现有两种方式, 分别是 FIFO(先进先出队列) 和
	// PriorityQueue(优先级队列). 其中优先级队列根据 Pod 资源对象的优先级进行排序, 优先级越高的排得越前.
	SchedulingQueue internalqueue.SchedulingQueue

	// Profiles are the scheduling profiles.
	Profiles profile.Map

	client clientset.Interface
}

// schedulerOptions 调度器选项
type schedulerOptions struct {
	// 调度器的算法源
	schedulerAlgorithmSource schedulerapi.SchedulerAlgorithmSource
	// percentageOfNodesToScore 是用于指定一个百分比值, 一旦发现可用于运行 Pod 的节点与所有节点的百分比值达到该指定的
	// 百分比值, 那么调度器将停止搜索集群中更多可运行该 Pod 的节点. 这有助于提高调度器的性能.
	// 注意: 调度器会始终尝试至少查找 "minFeasibleNodesToFind" 个可行节点, 而不管该标志设置的百分比值是多少.
	percentageOfNodesToScore int32
	// 不可调度的 Pods 的初始 backoff（退避） 秒数, 没有指定, 则默认为 1s
	podInitialBackoffSeconds int64
	// 不可调度的 Pods 的最大 backoff（退避） 秒数. 必须大于或等于 podInitialBackoffSeconds.
	// 没有指定, 则默认 10s.
	podMaxBackoffSeconds     int64
	// Contains out-of-tree plugins to be merged with the in-tree registry.
	// 包含要与 in-tree 注册表 registry 合并的树外插件.
	frameworkOutOfTreeRegistry frameworkruntime.Registry
	profiles                   []schedulerapi.KubeSchedulerProfile
	// 扩展调度器列表
	extenders                  []schedulerapi.Extender
	frameworkCapturer          FrameworkCapturer
}

// Option configures a Scheduler
type Option func(*schedulerOptions)

// WithProfiles sets profiles for Scheduler. By default, there is one profile
// with the name "default-scheduler".
//
// WithProfiles 设置调度器的 profiles. 默认情况下, 有一个名为 "default-scheduler" 的 profile.
func WithProfiles(p ...schedulerapi.KubeSchedulerProfile) Option {
	return func(o *schedulerOptions) {
		o.profiles = p
	}
}

// WithParallelism sets the parallelism for all scheduler algorithms. Default is 16.
// TODO(#95952): Remove global setter in favor of a struct that holds the configuration.
func WithParallelism(threads int32) Option {
	return func(o *schedulerOptions) {
		parallelize.SetParallelism(int(threads))
	}
}

// WithAlgorithmSource sets schedulerAlgorithmSource for Scheduler, the default is a source with DefaultProvider.
func WithAlgorithmSource(source schedulerapi.SchedulerAlgorithmSource) Option {
	return func(o *schedulerOptions) {
		o.schedulerAlgorithmSource = source
	}
}

// WithPercentageOfNodesToScore sets percentageOfNodesToScore for Scheduler, the default value is 50
// WithPercentageOfNodesToScore 为调度器设置 percentageOfNodesToScore, 默认 50
func WithPercentageOfNodesToScore(percentageOfNodesToScore int32) Option {
	return func(o *schedulerOptions) {
		o.percentageOfNodesToScore = percentageOfNodesToScore
	}
}

// WithFrameworkOutOfTreeRegistry sets the registry for out-of-tree plugins. Those plugins
// will be appended to the default registry.
func WithFrameworkOutOfTreeRegistry(registry frameworkruntime.Registry) Option {
	return func(o *schedulerOptions) {
		o.frameworkOutOfTreeRegistry = registry
	}
}

// WithPodInitialBackoffSeconds sets podInitialBackoffSeconds for Scheduler, the default value is 1
func WithPodInitialBackoffSeconds(podInitialBackoffSeconds int64) Option {
	return func(o *schedulerOptions) {
		o.podInitialBackoffSeconds = podInitialBackoffSeconds
	}
}

// WithPodMaxBackoffSeconds sets podMaxBackoffSeconds for Scheduler, the default value is 10
func WithPodMaxBackoffSeconds(podMaxBackoffSeconds int64) Option {
	return func(o *schedulerOptions) {
		o.podMaxBackoffSeconds = podMaxBackoffSeconds
	}
}

// WithExtenders sets extenders for the Scheduler
func WithExtenders(e ...schedulerapi.Extender) Option {
	return func(o *schedulerOptions) {
		o.extenders = e
	}
}

// FrameworkCapturer is used for registering a notify function in building framework.
//
// FrameworkCapturer 用于在构建框架（framework）中注册的通知功能.
type FrameworkCapturer func(schedulerapi.KubeSchedulerProfile)

// WithBuildFrameworkCapturer sets a notify function for getting buildFramework details.
func WithBuildFrameworkCapturer(fc FrameworkCapturer) Option {
	return func(o *schedulerOptions) {
		o.frameworkCapturer = fc
	}
}

var defaultSchedulerOptions = schedulerOptions{
	profiles: []schedulerapi.KubeSchedulerProfile{
		// Profiles' default plugins are set from the algorithm provider.
		{SchedulerName: v1.DefaultSchedulerName},
	},
	schedulerAlgorithmSource: schedulerapi.SchedulerAlgorithmSource{
		Provider: defaultAlgorithmSourceProviderName(),
	},
	percentageOfNodesToScore: schedulerapi.DefaultPercentageOfNodesToScore,
	podInitialBackoffSeconds: int64(internalqueue.DefaultPodInitialBackoffDuration.Seconds()),
	podMaxBackoffSeconds:     int64(internalqueue.DefaultPodMaxBackoffDuration.Seconds()),
}

// New returns a Scheduler
// New 实例化 Scheduler 对象
func New(client clientset.Interface,
	informerFactory informers.SharedInformerFactory,
	recorderFactory profile.RecorderFactory,
	stopCh <-chan struct{},
	opts ...Option) (*Scheduler, error) {

	stopEverything := stopCh
	if stopEverything == nil {
		stopEverything = wait.NeverStop
	}

	// 首先使用带有默认值的 schedulerOptions 结构体对象 defaultSchedulerOptions
	options := defaultSchedulerOptions
	// opts 是一系列传入的 Withxxx 回调函数, 这些函数是将 cc.ComponentConfig 中的相关配置覆盖 options 中的默认配置,
	// 这样便将用户通过命令行传递的所有选项与 kube-scheduler 自身的默认值进行了覆盖和融合. options 对象即为融合后的
	// 调度策略相关的参数.
	for _, opt := range opts {
		opt(&options)
	}

	// 构建调度器缓存 schedulerCache 对象, 并在一个 goroutine 中启动该 schedulerCache,
	// 用于定时清理过期的 assumedPod.
	schedulerCache := internalcache.New(30*time.Second, stopEverything)

	// 向 registry（注册表）注册所有的树内（in-tree）插件
	registry := frameworkplugins.NewInTreeRegistry()
	// 将树外插件合并到 registry 中
	if err := registry.Merge(options.frameworkOutOfTreeRegistry); err != nil {
		return nil, err
	}

	// 实例化一个空的 Snapshot 对象
	snapshot := internalcache.NewEmptySnapshot()

	configurator := &Configurator{
		client:                   client,
		recorderFactory:          recorderFactory,
		informerFactory:          informerFactory,
		schedulerCache:           schedulerCache,
		StopEverything:           stopEverything,
		percentageOfNodesToScore: options.percentageOfNodesToScore,
		podInitialBackoffSeconds: options.podInitialBackoffSeconds,
		podMaxBackoffSeconds:     options.podMaxBackoffSeconds,
		profiles:                 append([]schedulerapi.KubeSchedulerProfile(nil), options.profiles...),
		registry:                 registry,
		nodeInfoSnapshot:         snapshot,
		extenders:                options.extenders,
		frameworkCapturer:        options.frameworkCapturer,
	}

	// 注册所有的指标监控
	metrics.Register()

	// 实例化一个空的调度器对象
	var sched *Scheduler
	// 获取参数中指定的调度算法源
	source := options.schedulerAlgorithmSource
	switch {
	// 通过名称 Provider 的方式实例化调度器
	case source.Provider != nil:
		// Create the config from a named algorithm provider.
		sc, err := configurator.createFromProvider(*source.Provider)
		if err != nil {
			return nil, fmt.Errorf("couldn't create scheduler using provider %q: %v", *source.Provider, err)
		}
		sched = sc
	// 通过 --policy-config-file 参数指定调度策略文件
	case source.Policy != nil:
		// Create the config from a user specified policy source.
		policy := &schedulerapi.Policy{}
		switch {
		case source.Policy.File != nil:
			if err := initPolicyFromFile(source.Policy.File.Path, policy); err != nil {
				return nil, err
			}
		case source.Policy.ConfigMap != nil:
			if err := initPolicyFromConfigMap(client, source.Policy.ConfigMap, policy); err != nil {
				return nil, err
			}
		}
		// Set extenders on the configurator now that we've decoded the policy
		// In this case, c.extenders should be nil since we're using a policy (and therefore not componentconfig,
		// which would have set extenders in the above instantiation of Configurator from CC options)
		configurator.extenders = policy.Extenders
		sc, err := configurator.createFromConfig(*policy)
		if err != nil {
			return nil, fmt.Errorf("couldn't create scheduler from policy: %v", err)
		}
		sched = sc
	default:
		return nil, fmt.Errorf("unsupported algorithm source: %v", source)
	}
	// Additional tweaks to the config produced by the configurator.
	sched.StopEverything = stopEverything
	sched.client = client

	// 为所有 Informer 对象添加对资源事件的监控并设置回调函数
	addAllEventHandlers(sched, informerFactory)
	return sched, nil
}

// initPolicyFromFile initialize policy from file
func initPolicyFromFile(policyFile string, policy *schedulerapi.Policy) error {
	// Use a policy serialized in a file.
	_, err := os.Stat(policyFile)
	if err != nil {
		return fmt.Errorf("missing policy config file %s", policyFile)
	}
	data, err := ioutil.ReadFile(policyFile)
	if err != nil {
		return fmt.Errorf("couldn't read policy config: %v", err)
	}
	err = runtime.DecodeInto(scheme.Codecs.UniversalDecoder(), []byte(data), policy)
	if err != nil {
		return fmt.Errorf("invalid policy: %v", err)
	}
	return nil
}

// initPolicyFromConfigMap initialize policy from configMap
func initPolicyFromConfigMap(client clientset.Interface, policyRef *schedulerapi.SchedulerPolicyConfigMapSource, policy *schedulerapi.Policy) error {
	// Use a policy serialized in a config map value.
	policyConfigMap, err := client.CoreV1().ConfigMaps(policyRef.Namespace).Get(context.TODO(), policyRef.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("couldn't get policy config map %s/%s: %v", policyRef.Namespace, policyRef.Name, err)
	}
	data, found := policyConfigMap.Data[schedulerapi.SchedulerPolicyConfigMapKey]
	if !found {
		return fmt.Errorf("missing policy config map value at key %q", schedulerapi.SchedulerPolicyConfigMapKey)
	}
	err = runtime.DecodeInto(scheme.Codecs.UniversalDecoder(), []byte(data), policy)
	if err != nil {
		return fmt.Errorf("invalid policy: %v", err)
	}
	return nil
}

// Run begins watching and scheduling. It starts scheduling and blocked until the context is done.
//
// Run 开始监控和调度. 它开始调度并阻塞, 直到 ctx 终止.
func (sched *Scheduler) Run(ctx context.Context) {
	// 启动调度队列
	sched.SchedulingQueue.Run()
	// sched.scheduleOne 是 kube-scheduler 组件的调度主逻辑, 它通过 wait.UntilWithContext 定时器执行.
	// 内部会定时调用 sched.scheduleOne 函数.
	wait.UntilWithContext(ctx, sched.scheduleOne, 0)
	sched.SchedulingQueue.Close()
}

// recordSchedulingFailure records an event for the pod that indicates the
// pod has failed to schedule. Also, update the pod condition and nominated node name if set.
func (sched *Scheduler) recordSchedulingFailure(fwk framework.Framework, podInfo *framework.QueuedPodInfo, err error, reason string, nominatedNode string) {
	sched.Error(podInfo, err)

	// Update the scheduling queue with the nominated pod information. Without
	// this, there would be a race condition between the next scheduling cycle
	// and the time the scheduler receives a Pod Update for the nominated pod.
	// Here we check for nil only for tests.
	if sched.SchedulingQueue != nil {
		sched.SchedulingQueue.AddNominatedPod(podInfo.Pod, nominatedNode)
	}

	pod := podInfo.Pod
	fwk.EventRecorder().Eventf(pod, nil, v1.EventTypeWarning, "FailedScheduling", "Scheduling", err.Error())
	if err := updatePod(sched.client, pod, &v1.PodCondition{
		Type:    v1.PodScheduled,
		Status:  v1.ConditionFalse,
		Reason:  reason,
		Message: err.Error(),
	}, nominatedNode); err != nil {
		klog.Errorf("Error updating pod %s/%s: %v", pod.Namespace, pod.Name, err)
	}
}

func updatePod(client clientset.Interface, pod *v1.Pod, condition *v1.PodCondition, nominatedNode string) error {
	klog.V(3).Infof("Updating pod condition for %s/%s to (%s==%s, Reason=%s)", pod.Namespace, pod.Name, condition.Type, condition.Status, condition.Reason)
	podCopy := pod.DeepCopy()
	// NominatedNodeName is updated only if we are trying to set it, and the value is
	// different from the existing one.
	if !podutil.UpdatePodCondition(&podCopy.Status, condition) &&
		(len(nominatedNode) == 0 || pod.Status.NominatedNodeName == nominatedNode) {
		return nil
	}
	if nominatedNode != "" {
		podCopy.Status.NominatedNodeName = nominatedNode
	}
	return util.PatchPod(client, pod, podCopy)
}

// assume signals to the cache that a pod is already in the cache, so that binding can be asynchronous.
// assume modifies `assumed`.
func (sched *Scheduler) assume(assumed *v1.Pod, host string) error {
	// Optimistically assume that the binding will succeed and send it to apiserver
	// in the background.
	// If the binding fails, scheduler will release resources allocated to assumed pod
	// immediately.
	assumed.Spec.NodeName = host

	if err := sched.SchedulerCache.AssumePod(assumed); err != nil {
		klog.Errorf("scheduler cache AssumePod failed: %v", err)
		return err
	}
	// if "assumed" is a nominated pod, we should remove it from internal cache
	if sched.SchedulingQueue != nil {
		sched.SchedulingQueue.DeleteNominatedPodIfExists(assumed)
	}

	return nil
}

// bind binds a pod to a given node defined in a binding object.
// The precedence for binding is: (1) extenders and (2) framework plugins.
// We expect this to run asynchronously, so we handle binding metrics internally.
//
// 当调度器为 Pod 资源对象选择了一个合适的节点时, 通过 sched.bind 函数将合适的节点与 Pod 资源对象绑定在一起.
// 该过程是异步的, 无须等待 bind 操作完成即可进入下一轮的调度周期.
// 由 kube-scheduler 调度器通过 ClientSet 向 kube-apiserver 发送 v1.Binding 资源对象, 如果绑定失败, 则执行回滚操作;
// 如果绑定成功, 则当前调度周期完成, 后面运行 Pod 资源对象的工作交给绑定节点上的 kubelet 组件.
func (sched *Scheduler) bind(ctx context.Context, fwk framework.Framework, assumed *v1.Pod, targetNode string, state *framework.CycleState) (err error) {
	start := time.Now()
	defer func() {
		sched.finishBinding(fwk, assumed, targetNode, start, err)
	}()

	bound, err := sched.extendersBinding(assumed, targetNode)
	if bound {
		return err
	}
	bindStatus := fwk.RunBindPlugins(ctx, state, assumed, targetNode)
	if bindStatus.IsSuccess() {
		return nil
	}
	if bindStatus.Code() == framework.Error {
		return bindStatus.AsError()
	}
	return fmt.Errorf("bind status: %s, %v", bindStatus.Code().String(), bindStatus.Message())
}

// TODO(#87159): Move this to a Plugin.
func (sched *Scheduler) extendersBinding(pod *v1.Pod, node string) (bool, error) {
	for _, extender := range sched.Algorithm.Extenders() {
		if !extender.IsBinder() || !extender.IsInterested(pod) {
			continue
		}
		return true, extender.Bind(&v1.Binding{
			ObjectMeta: metav1.ObjectMeta{Namespace: pod.Namespace, Name: pod.Name, UID: pod.UID},
			Target:     v1.ObjectReference{Kind: "Node", Name: node},
		})
	}
	return false, nil
}

func (sched *Scheduler) finishBinding(fwk framework.Framework, assumed *v1.Pod, targetNode string, start time.Time, err error) {
	if finErr := sched.SchedulerCache.FinishBinding(assumed); finErr != nil {
		klog.Errorf("scheduler cache FinishBinding failed: %v", finErr)
	}
	if err != nil {
		klog.V(1).Infof("Failed to bind pod: %v/%v", assumed.Namespace, assumed.Name)
		return
	}

	metrics.DeprecatedBindingLatency.Observe(metrics.SinceInSeconds(start))
	fwk.EventRecorder().Eventf(assumed, nil, v1.EventTypeNormal, "Scheduled", "Binding", "Successfully assigned %v/%v to %v", assumed.Namespace, assumed.Name, targetNode)
}

// scheduleOne does the entire scheduling workflow for a single pod. It is serialized on the scheduling algorithm's host fitting.
func (sched *Scheduler) scheduleOne(ctx context.Context) {
	// 从优先级队列中获取一个优先级最高的待调度的 Pod 资源对象, 该过程是阻塞模式的, 当优先级队列中
	// 不存在任何 Pod 资源对象时, sched.NextPod 函数处于等待状态.
	podInfo := sched.NextPod()
	// pod could be nil when schedulerQueue is closed
	if podInfo == nil || podInfo.Pod == nil {
		return
	}
	pod := podInfo.Pod
	fwk, err := sched.frameworkForPod(pod)
	if err != nil {
		// This shouldn't happen, because we only accept for scheduling the pods
		// which specify a scheduler name that matches one of the profiles.
		klog.Error(err)
		return
	}
	if sched.skipPodSchedule(fwk, pod) {
		return
	}

	klog.V(3).Infof("Attempting to schedule pod: %v/%v", pod.Namespace, pod.Name)

	// Synchronously attempt to find a fit for the pod.
	start := time.Now()
	state := framework.NewCycleState()
	state.SetRecordPluginMetrics(rand.Intn(100) < pluginMetricsSamplePercent)
	// 创建调度循环上下文对象
	schedulingCycleCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	// 执行预选调度算法和优选调度算法, 为 Pod 资源对象选择一个合适的节点.
	scheduleResult, err := sched.Algorithm.Schedule(schedulingCycleCtx, fwk, state, pod)
	if err != nil {
		// Schedule() may have failed because the pod would not fit on any host, so we try to
		// preempt, with the expectation that the next time the pod is tried for scheduling it
		// will fit due to the preemption. It is also possible that a different pod will schedule
		// into the resources that were preempted, but this is harmless.
		//
		// Schedule() 可能会失败, 因为没有找到符合条件的节点, 因此会尝试进行抢占.
		//
		// 抢占机制:
		// 当高优先级的 Pod 资源对象没有找到合适的节点时, kube-scheduler 调度器会尝试抢占低优先级的 Pod 资源对象的节点.
		// 抢占过程是将低优先级的 Pod 资源对象从所在的节点上驱逐走, 再让高优先级的 Pod 资源对象运行在该节点上, 被驱逐走的
		// 低优先级的 Pod 资源对象会重新进入调度队列并等待再次选择合适的节点.
		nominatedNode := ""
		if fitError, ok := err.(*core.FitError); ok {
			// 如果没有注册 PostFilter 插件, 则不会执行抢占机制
			if !fwk.HasPostFilterPlugins() {
				klog.V(3).Infof("No PostFilter plugins are registered, so no preemption will be performed.")
			} else {
				// Run PostFilter plugins to try to make the pod schedulable in a future scheduling cycle.
				// 运行 PostFilter plugins 来执行抢占机制
				result, status := fwk.RunPostFilterPlugins(ctx, state, pod, fitError.FilteredNodesStatuses)
				if status.Code() == framework.Error {
					klog.Errorf("Status after running PostFilter plugins for pod %v/%v: %v", pod.Namespace, pod.Name, status)
				} else {
					klog.V(5).Infof("Status after running PostFilter plugins for pod %v/%v: %v", pod.Namespace, pod.Name, status)
				}
				if status.IsSuccess() && result != nil {
					nominatedNode = result.NominatedNodeName
				}
			}
			// Pod did not fit anywhere, so it is counted as a failure. If preemption
			// succeeds, the pod should get counted as a success the next time we try to
			// schedule it. (hopefully)
			metrics.PodUnschedulable(fwk.ProfileName(), metrics.SinceInSeconds(start))
		} else if err == core.ErrNoNodesAvailable {
			// No nodes available is counted as unschedulable rather than an error.
			metrics.PodUnschedulable(fwk.ProfileName(), metrics.SinceInSeconds(start))
		} else {
			klog.ErrorS(err, "Error selecting node for pod", "pod", klog.KObj(pod))
			metrics.PodScheduleError(fwk.ProfileName(), metrics.SinceInSeconds(start))
		}
		sched.recordSchedulingFailure(fwk, podInfo, err, v1.PodReasonUnschedulable, nominatedNode)
		return
	}
	metrics.SchedulingAlgorithmLatency.Observe(metrics.SinceInSeconds(start))
	// Tell the cache to assume that a pod now is running on a given node, even though it hasn't been bound yet.
	// This allows us to keep scheduling without waiting on binding to occur.
	assumedPodInfo := podInfo.DeepCopy()
	assumedPod := assumedPodInfo.Pod
	// assume modifies `assumedPod` by setting NodeName=scheduleResult.SuggestedHost
	err = sched.assume(assumedPod, scheduleResult.SuggestedHost)
	if err != nil {
		metrics.PodScheduleError(fwk.ProfileName(), metrics.SinceInSeconds(start))
		// This is most probably result of a BUG in retrying logic.
		// We report an error here so that pod scheduling can be retried.
		// This relies on the fact that Error will check if the pod has been bound
		// to a node and if so will not add it back to the unscheduled pods queue
		// (otherwise this would cause an infinite loop).
		sched.recordSchedulingFailure(fwk, assumedPodInfo, err, SchedulerError, "")
		return
	}

	// Run the Reserve method of reserve plugins.
	if sts := fwk.RunReservePluginsReserve(schedulingCycleCtx, state, assumedPod, scheduleResult.SuggestedHost); !sts.IsSuccess() {
		metrics.PodScheduleError(fwk.ProfileName(), metrics.SinceInSeconds(start))
		// trigger un-reserve to clean up state associated with the reserved Pod
		fwk.RunReservePluginsUnreserve(schedulingCycleCtx, state, assumedPod, scheduleResult.SuggestedHost)
		if forgetErr := sched.SchedulerCache.ForgetPod(assumedPod); forgetErr != nil {
			klog.Errorf("scheduler cache ForgetPod failed: %v", forgetErr)
		}
		sched.recordSchedulingFailure(fwk, assumedPodInfo, sts.AsError(), SchedulerError, "")
		return
	}

	// Run "permit" plugins.
	runPermitStatus := fwk.RunPermitPlugins(schedulingCycleCtx, state, assumedPod, scheduleResult.SuggestedHost)
	if runPermitStatus.Code() != framework.Wait && !runPermitStatus.IsSuccess() {
		var reason string
		if runPermitStatus.IsUnschedulable() {
			metrics.PodUnschedulable(fwk.ProfileName(), metrics.SinceInSeconds(start))
			reason = v1.PodReasonUnschedulable
		} else {
			metrics.PodScheduleError(fwk.ProfileName(), metrics.SinceInSeconds(start))
			reason = SchedulerError
		}
		// One of the plugins returned status different than success or wait.
		fwk.RunReservePluginsUnreserve(schedulingCycleCtx, state, assumedPod, scheduleResult.SuggestedHost)
		if forgetErr := sched.SchedulerCache.ForgetPod(assumedPod); forgetErr != nil {
			klog.Errorf("scheduler cache ForgetPod failed: %v", forgetErr)
		}
		sched.recordSchedulingFailure(fwk, assumedPodInfo, runPermitStatus.AsError(), reason, "")
		return
	}

	// bind the pod to its host asynchronously (we can do this b/c of the assumption step above).
	go func() {
		bindingCycleCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		metrics.SchedulerGoroutines.WithLabelValues("binding").Inc()
		defer metrics.SchedulerGoroutines.WithLabelValues("binding").Dec()

		waitOnPermitStatus := fwk.WaitOnPermit(bindingCycleCtx, assumedPod)
		if !waitOnPermitStatus.IsSuccess() {
			var reason string
			if waitOnPermitStatus.IsUnschedulable() {
				metrics.PodUnschedulable(fwk.ProfileName(), metrics.SinceInSeconds(start))
				reason = v1.PodReasonUnschedulable
			} else {
				metrics.PodScheduleError(fwk.ProfileName(), metrics.SinceInSeconds(start))
				reason = SchedulerError
			}
			// trigger un-reserve plugins to clean up state associated with the reserved Pod
			fwk.RunReservePluginsUnreserve(bindingCycleCtx, state, assumedPod, scheduleResult.SuggestedHost)
			if forgetErr := sched.SchedulerCache.ForgetPod(assumedPod); forgetErr != nil {
				klog.Errorf("scheduler cache ForgetPod failed: %v", forgetErr)
			}
			sched.recordSchedulingFailure(fwk, assumedPodInfo, waitOnPermitStatus.AsError(), reason, "")
			return
		}

		// Run "prebind" plugins.
		preBindStatus := fwk.RunPreBindPlugins(bindingCycleCtx, state, assumedPod, scheduleResult.SuggestedHost)
		if !preBindStatus.IsSuccess() {
			metrics.PodScheduleError(fwk.ProfileName(), metrics.SinceInSeconds(start))
			// trigger un-reserve plugins to clean up state associated with the reserved Pod
			fwk.RunReservePluginsUnreserve(bindingCycleCtx, state, assumedPod, scheduleResult.SuggestedHost)
			if forgetErr := sched.SchedulerCache.ForgetPod(assumedPod); forgetErr != nil {
				klog.Errorf("scheduler cache ForgetPod failed: %v", forgetErr)
			}
			sched.recordSchedulingFailure(fwk, assumedPodInfo, preBindStatus.AsError(), SchedulerError, "")
			return
		}

		// 当调度器为 Pod 资源对象选择了一个合适的节点时, 通过 sched.bind 函数将合适的节点与 Pod 资源对象绑定在一起.
		// 该过程是异步的, 无须等待 bind 操作完成即可进入下一轮的调度周期.
		err := sched.bind(bindingCycleCtx, fwk, assumedPod, scheduleResult.SuggestedHost, state)
		if err != nil {
			metrics.PodScheduleError(fwk.ProfileName(), metrics.SinceInSeconds(start))
			// trigger un-reserve plugins to clean up state associated with the reserved Pod
			fwk.RunReservePluginsUnreserve(bindingCycleCtx, state, assumedPod, scheduleResult.SuggestedHost)
			if err := sched.SchedulerCache.ForgetPod(assumedPod); err != nil {
				klog.Errorf("scheduler cache ForgetPod failed: %v", err)
			}
			sched.recordSchedulingFailure(fwk, assumedPodInfo, fmt.Errorf("binding rejected: %w", err), SchedulerError, "")
		} else {
			// Calculating nodeResourceString can be heavy. Avoid it if klog verbosity is below 2.
			if klog.V(2).Enabled() {
				klog.InfoS("Successfully bound pod to node", "pod", klog.KObj(pod), "node", scheduleResult.SuggestedHost, "evaluatedNodes", scheduleResult.EvaluatedNodes, "feasibleNodes", scheduleResult.FeasibleNodes)
			}
			metrics.PodScheduled(fwk.ProfileName(), metrics.SinceInSeconds(start))
			metrics.PodSchedulingAttempts.Observe(float64(podInfo.Attempts))
			metrics.PodSchedulingDuration.WithLabelValues(getAttemptsLabel(podInfo)).Observe(metrics.SinceInSeconds(podInfo.InitialAttemptTimestamp))

			// Run "postbind" plugins.
			fwk.RunPostBindPlugins(bindingCycleCtx, state, assumedPod, scheduleResult.SuggestedHost)
		}
	}()
}

func getAttemptsLabel(p *framework.QueuedPodInfo) string {
	// We breakdown the pod scheduling duration by attempts capped to a limit
	// to avoid ending up with a high cardinality metric.
	if p.Attempts >= 15 {
		return "15+"
	}
	return strconv.Itoa(p.Attempts)
}

func (sched *Scheduler) frameworkForPod(pod *v1.Pod) (framework.Framework, error) {
	fwk, ok := sched.Profiles[pod.Spec.SchedulerName]
	if !ok {
		return nil, fmt.Errorf("profile not found for scheduler name %q", pod.Spec.SchedulerName)
	}
	return fwk, nil
}

// skipPodSchedule returns true if we could skip scheduling the pod for specified cases.
func (sched *Scheduler) skipPodSchedule(fwk framework.Framework, pod *v1.Pod) bool {
	// Case 1: pod is being deleted.
	if pod.DeletionTimestamp != nil {
		fwk.EventRecorder().Eventf(pod, nil, v1.EventTypeWarning, "FailedScheduling", "Scheduling", "skip schedule deleting pod: %v/%v", pod.Namespace, pod.Name)
		klog.V(3).Infof("Skip schedule deleting pod: %v/%v", pod.Namespace, pod.Name)
		return true
	}

	// Case 2: pod has been assumed and pod updates could be skipped.
	// An assumed pod can be added again to the scheduling queue if it got an update event
	// during its previous scheduling cycle but before getting assumed.
	if sched.skipPodUpdate(pod) {
		return true
	}

	return false
}

func defaultAlgorithmSourceProviderName() *string {
	provider := schedulerapi.SchedulerDefaultProviderName
	return &provider
}

// NewInformerFactory creates a SharedInformerFactory and initializes a scheduler specific
// in-place podInformer.
func NewInformerFactory(cs clientset.Interface, resyncPeriod time.Duration) informers.SharedInformerFactory {
	informerFactory := informers.NewSharedInformerFactory(cs, resyncPeriod)
	informerFactory.InformerFor(&v1.Pod{}, newPodInformer)
	return informerFactory
}

// newPodInformer creates a shared index informer that returns only non-terminal pods.
func newPodInformer(cs clientset.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	selector := fmt.Sprintf("status.phase!=%v,status.phase!=%v", v1.PodSucceeded, v1.PodFailed)
	tweakListOptions := func(options *metav1.ListOptions) {
		options.FieldSelector = selector
	}
	return coreinformers.NewFilteredPodInformer(cs, metav1.NamespaceAll, resyncPeriod, nil, tweakListOptions)
}
