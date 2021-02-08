/*
Copyright 2018 The Kubernetes Authors.

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

package config

import (
	"math"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	componentbaseconfig "k8s.io/component-base/config"
)

const (
	// SchedulerDefaultLockObjectNamespace defines default scheduler lock object namespace ("kube-system")
	SchedulerDefaultLockObjectNamespace string = metav1.NamespaceSystem

	// SchedulerDefaultLockObjectName defines default scheduler lock object name ("kube-scheduler")
	SchedulerDefaultLockObjectName = "kube-scheduler"

	// SchedulerPolicyConfigMapKey defines the key of the element in the
	// scheduler's policy ConfigMap that contains scheduler's policy config.
	SchedulerPolicyConfigMapKey = "policy.cfg"

	// SchedulerDefaultProviderName defines the default provider names
	// 定义默认的调度器算法 provider
	SchedulerDefaultProviderName = "DefaultProvider"

	// DefaultInsecureSchedulerPort is the default port for the scheduler status server.
	// May be overridden by a flag at startup.
	// Deprecated: use the secure KubeSchedulerPort instead.
	DefaultInsecureSchedulerPort = 10251

	// DefaultKubeSchedulerPort is the default port for the scheduler status server.
	// May be overridden by a flag at startup.
	//
	// DefaultKubeSchedulerPort 是 kube-scheduler 服务器默认绑定的端口. 可能会被启动参数覆盖.
	DefaultKubeSchedulerPort = 10259
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KubeSchedulerConfiguration configures a scheduler
//
// KubeSchedulerConfiguration 调度器的配置
type KubeSchedulerConfiguration struct {
	metav1.TypeMeta

	// Parallelism defines the amount of parallelism in algorithms for scheduling a Pods. Must be greater than 0. Defaults to 16
	//
	// Parallelism 定义在调度 Pod 的算法中并发调度的数量. 必须大于 0, 默认为 16.
	Parallelism int32

	// AlgorithmSource specifies the scheduler algorithm source.
	// TODO(#87526): Remove AlgorithmSource from this package
	// DEPRECATED: AlgorithmSource is removed in the v1beta1 ComponentConfig
	//
	// AlgorithmSource 指定调度算法源.
	// TODO(#87526): 从该 package 中移除 AlgorithmSource
	// 已淘汰: AlgorithmSource 已在 v1beta1 版本的 ComponentConfig 中移除
	AlgorithmSource SchedulerAlgorithmSource

	// LeaderElection defines the configuration of leader election client.
	//
	// LeaderElection 定义了 leader 选举客户端的配置
	LeaderElection componentbaseconfig.LeaderElectionConfiguration

	// ClientConnection specifies the kubeconfig file and client connection
	// settings for the proxy server to use when communicating with the apiserver.
	//
	// ClientConnection 指定代理服务器(proxy server) 与 kube-apiserver 交互时使用的 kubeconfig 文件
	// 和客户端连接设置.
	ClientConnection componentbaseconfig.ClientConnectionConfiguration
	// HealthzBindAddress is the IP address and port for the health check server to serve on,
	// defaulting to 0.0.0.0:10251
	//
	// HealthzBindAddress 是健康检查服务器运行时的 IP 地址和端口, 默认为 0.0.0.0:10251.
	HealthzBindAddress string
	// MetricsBindAddress is the IP address and port for the metrics server to
	// serve on, defaulting to 0.0.0.0:10251.
	//
	// MetricsBindAddress 是 metric 服务器运行时监听的 IP 地址和端口, 默认为 0.0.0.0:10251.
	MetricsBindAddress string

	// DebuggingConfiguration holds configuration for Debugging related features
	// TODO: We might wanna make this a substruct like Debugging componentbaseconfig.DebuggingConfiguration
	//
	// DebuggingConfiguration 保存用于调试相关功能的配置.
	// 备注: 我们可能想它成为 "Debugging componentbaseconfig.DebuggingConfiguration" 这样的子结构.
	componentbaseconfig.DebuggingConfiguration

	// PercentageOfNodesToScore is the percentage of all nodes that once found feasible
	// for running a pod, the scheduler stops its search for more feasible nodes in
	// the cluster. This helps improve scheduler's performance. Scheduler always tries to find
	// at least "minFeasibleNodesToFind" feasible nodes no matter what the value of this flag is.
	// Example: if the cluster size is 500 nodes and the value of this flag is 30,
	// then scheduler stops finding further feasible nodes once it finds 150 feasible ones.
	// When the value is 0, default percentage (5%--50% based on the size of the cluster) of the
	// nodes will be scored.
	//
	// PercentageOfNodesToScore 是用于指定一个百分比值, 一旦发现可用于运行 Pod 的节点与所有节点的百分比值达到该指定的
	// 百分比值, 那么调度器将停止搜索集群中更多可运行该 Pod 的节点. 这有助于提高调度器的性能.
	// 注意: 调度器会始终尝试至少查找 "minFeasibleNodesToFind" 个可行节点, 而不管该标志设置的百分比值是多少.
	PercentageOfNodesToScore int32

	// PodInitialBackoffSeconds is the initial backoff for unschedulable pods.
	// If specified, it must be greater than 0. If this value is null, the default value (1s)
	// will be used.
	//
	// PodInitialBackoffSeconds 是不可调度的 Pods 的初始 backoff（退避）秒数. 如果指定, 则必须大于 0. 如果该值为 null,
	// 则将使用默认值 (1s).
	PodInitialBackoffSeconds int64

	// PodMaxBackoffSeconds is the max backoff for unschedulable pods.
	// If specified, it must be greater than or equal to podInitialBackoffSeconds. If this value is null,
	// the default value (10s) will be used.
	//
	// PodMaxBackoffSeconds 是不可调度的 Pods 的最大 backoff（退避） 秒数. 如果指定, 则必须大于或等于
	// PodInitialBackoffSeconds. 如果该值为 null, 则将使用默认值 10s.
	PodMaxBackoffSeconds int64

	// Profiles are scheduling profiles that kube-scheduler supports. Pods can
	// choose to be scheduled under a particular profile by setting its associated
	// scheduler name. Pods that don't specify any scheduler name are scheduled
	// with the "default-scheduler" profile, if present here.
	Profiles []KubeSchedulerProfile

	// Extenders are the list of scheduler extenders, each holding the values of how to communicate
	// with the extender. These extenders are shared by all scheduler profiles.
	//
	// Extenders 是调度器的 extenders 列表 (extender 即扩展调度器, 意思是启动 kube-scheduler 后, 再启动这里定义的扩展调度器),
	// 每个 extender 都定义了如何与 kube-scheduler 进行通信的值. 这些 extenders 由所有 scheduler profiles 共享.
	Extenders []Extender
}

// KubeSchedulerProfile is a scheduling profile.
//
// KubeSchedulerProfile 是调度器的 profile.
type KubeSchedulerProfile struct {
	// SchedulerName is the name of the scheduler associated to this profile.
	// If SchedulerName matches with the pod's "spec.schedulerName", then the pod
	// is scheduled with this profile.
	//
	// SchedulerName 是与该 profile 关联的调度器名称. 如果 SchedulerName 匹配 pod 的 "spec.schedulerName" 值,
	// 则将使用该 profile 调度 pod.
	SchedulerName string

	// Plugins specify the set of plugins that should be enabled or disabled.
	// Enabled plugins are the ones that should be enabled in addition to the
	// default plugins. Disabled plugins are any of the default plugins that
	// should be disabled.
	// When no enabled or disabled plugin is specified for an extension point,
	// default plugins for that extension point will be used if there is any.
	// If a QueueSort plugin is specified, the same QueueSort Plugin and
	// PluginConfig must be specified for all profiles.
	//
	// Plugins 指定启用/禁用的插件集. 启用的插件是除默认插件外还应启用的插件. 禁用的插件是应禁用的任意默认插件.
	// 如果没有为扩展点指定启用/禁用的插件, 那么将使用该扩展点的默认插件. 如果指定了 QueueSort 插件, 则必须为
	// 所有 profiles 文件指定相同的 QueueSort 插件和 PluginConfig.
	Plugins *Plugins

	// PluginConfig is an optional set of custom plugin arguments for each plugin.
	// Omitting config args for a plugin is equivalent to using the default config
	// for that plugin.
	//
	// PluginConfig 是每个插件的一组可选的自定义插件参数. 省略插件的配置参数等价于使用该插件的默认配置.
	PluginConfig []PluginConfig
}

// SchedulerAlgorithmSource is the source of a scheduler algorithm. One source
// field must be specified, and source fields are mutually exclusive.
//
// SchedulerAlgorithmSource 是调度算法源. 必须指定一个 source 字段, 且 source 字段是互斥的.
// 如下有两种指定调度算法的方式.
type SchedulerAlgorithmSource struct {
	// Policy is a policy based algorithm source.
	//
	// Policy 通过定义好的 Policy(策略) 资源的方式实例化调度算法函数. 该方式可通过
	// --policy-config-file 参数指定调度策略文件.
	Policy *SchedulerPolicySource
	// Provider is the name of a scheduling algorithm provider to use.
	//
	// 通用调度器, 通过名称的方式实例化调度算法函数, 这也是 kube-scheduler 的默认方式.
	Provider *string
}

// SchedulerPolicySource configures a means to obtain a scheduler Policy. One
// source field must be specified, and source fields are mutually exclusive.
type SchedulerPolicySource struct {
	// File is a file policy source.
	File *SchedulerPolicyFileSource
	// ConfigMap is a config map policy source.
	ConfigMap *SchedulerPolicyConfigMapSource
}

// SchedulerPolicyFileSource is a policy serialized to disk and accessed via
// path.
type SchedulerPolicyFileSource struct {
	// Path is the location of a serialized policy.
	Path string
}

// SchedulerPolicyConfigMapSource is a policy serialized into a config map value
// under the SchedulerPolicyConfigMapKey key.
type SchedulerPolicyConfigMapSource struct {
	// Namespace is the namespace of the policy config map.
	// 指定调度策略的 configmap 所在的命名空间
	Namespace string
	// Name is the name of the policy config map.
	// configmap 的名称
	Name string
}

// Plugins include multiple extension points. When specified, the list of plugins for
// a particular extension point are the only ones enabled. If an extension point is
// omitted from the config, then the default set of plugins is used for that extension point.
// Enabled plugins are called in the order specified here, after default plugins. If they need to
// be invoked before default plugins, default plugins must be disabled and re-enabled here in desired order.
//
// Plugins 包括多个扩展点. 指定后, 将启动特定扩展点的插件列表. 如果配置中忽略该扩展点, 则该扩展点将使用默认的插件列表.
// 在默认插件后, 将按这里指定的顺序调用已启用的插件. 如果需要在默认的插件列表前调用它们, 则必须禁用默认插件列表, 然后在这里
// 按序重新启用它们.
type Plugins struct {
	// QueueSort is a list of plugins that should be invoked when sorting pods in the scheduling queue.
	//
	// QueueSort 是对调度队列中的 Pod 进行排序时调用的插件列表, 即支持自定义的 Pod 的排序.
	QueueSort *PluginSet

	// PreFilter is a list of plugins that should be invoked at "PreFilter" extension point of the scheduling framework.
	//
	// PreFilter 是在调度流程中的名为 "PreFilter" 扩展点处调用的插件列表, 用于对 Pod 的请求做预处理.
	PreFilter *PluginSet

	// Filter is a list of plugins that should be invoked when filtering out nodes that cannot run the Pod.
	//
	// Filter 是在无法过滤出可以运行 Pod 的节点时调用的插件列表, 即自定义 Filter 逻辑.
	Filter *PluginSet

	// PostFilter is a list of plugins that are invoked after filtering phase, no matter whether filtering succeeds or not.
	//
	// PostFilter 是在过滤阶段 (filtering phase) 后调用的插件列表, 无论过滤是否成功.
	// 可以用于 logs/metrics, 或者对 Score 之前做数据预处理.
	PostFilter *PluginSet

	// PreScore is a list of plugins that are invoked before scoring.
	//
	// PreScore 是在打分前调用的插件列表.
	PreScore *PluginSet

	// Score is a list of plugins that should be invoked when ranking nodes that have passed the filtering phase.
	//
	// Score 是对已经通过过滤阶段的节点进行排名打分时调用的插件列表. (即自定义的 Score 逻辑)
	Score *PluginSet

	// Reserve is a list of plugins invoked when reserving/unreserving resources
	// after a node is assigned to run the pod.
	//
	// Reserve 是在将 Pod 分配给某个节点后 reserving/unreserving 资源时调用的插件列表.
	// 即有状态的 plugin 可以对资源做内存记账.
	Reserve *PluginSet

	// Permit is a list of plugins that control binding of a Pod. These plugins can prevent or delay binding of a Pod.
	//
	// Permit 是控制 Pod 绑定的插件列表. 这些 plugins 可以阻止或延迟 Pod 的绑定.
	// 即 wait, deny, approve, 可作为 gang 的插入点.
	Permit *PluginSet

	// PreBind is a list of plugins that should be invoked before a pod is bound.
	//
	// PreBind 是绑定 Pod 前调用的 plugins. 在真正 bind node 前, 执行一些操作, 如: 云盘挂载盘到 Node 上.
	PreBind *PluginSet

	// Bind is a list of plugins that should be invoked at "Bind" extension point of the scheduling framework.
	// The scheduler call these plugins in order. Scheduler skips the rest of these plugins as soon as one returns success.
	//
	// Bind 是在调度流程中的 "Bind" 扩展点调用的 plugin 列表. 调度器按顺序调用这些 plugins. 一旦有一个返回成功,
	// 调度器就会跳过其余的这些 plugins. 即一个 Pod 只会被一个 BindPlugin 处理.
	Bind *PluginSet

	// PostBind is a list of plugins that should be invoked after a pod is successfully bound.
	//
	// PostBind 是在成功绑定 Pod 后调用的 plugins.
	PostBind *PluginSet
}

// PluginSet specifies enabled and disabled plugins for an extension point.
// If an array is empty, missing, or nil, default plugins at that extension point will be used.
//
// PluginSet 为扩展点指定启用/禁用的 plugins. 如果数组为空, 缺失 或为 nil, 则在该扩展点中将使用默认 plugins.
type PluginSet struct {
	// Enabled specifies plugins that should be enabled in addition to default plugins.
	// These are called after default plugins and in the same order specified here.
	//
	// Enabled 指定除默认 plugins 外还应启用的 plugins. 这些额外的 plugins 将在调用完默认的 plugins 后按
	// 这里排列的顺序依次调用.
	Enabled []Plugin
	// Disabled specifies default plugins that should be disabled.
	// When all default plugins need to be disabled, an array containing only one "*" should be provided.
	//
	// Disabled 指定需要禁用的默认 plugins. 若需要禁用所有的默认 plugins, 则该数组将仅存在一个 "*".
	Disabled []Plugin
}

// Plugin specifies a plugin name and its weight when applicable. Weight is used only for Score plugins.
//
// Plugin 指定 plugin（插件）的名称及其使用时它的权重. weight（权重）仅用于 Score（打分）插件.
type Plugin struct {
	// Name defines the name of plugin
	// Name 定义插件的名称
	Name string
	// Weight defines the weight of plugin, only used for Score plugins.
	// Weight 定义插件的权重, 其仅用于 Score 插件.
	Weight int32
}

// PluginConfig specifies arguments that should be passed to a plugin at the time of initialization.
// A plugin that is invoked at multiple extension points is initialized once. Args can have arbitrary structure.
// It is up to the plugin to process these Args.
type PluginConfig struct {
	// Name defines the name of plugin being configured
	Name string
	// Args defines the arguments passed to the plugins at the time of initialization. Args can have arbitrary structure.
	Args runtime.Object
}

/*
 * NOTE: The following variables and methods are intentionally left out of the staging mirror.
 */
const (
	// DefaultPercentageOfNodesToScore defines the percentage of nodes of all nodes
	// that once found feasible, the scheduler stops looking for more nodes.
	// A value of 0 means adaptive, meaning the scheduler figures out a proper default.
	DefaultPercentageOfNodesToScore = 0

	// MaxCustomPriorityScore is the max score UtilizationShapePoint expects.
	MaxCustomPriorityScore int64 = 10

	// MaxTotalScore is the maximum total score.
	MaxTotalScore int64 = math.MaxInt64

	// MaxWeight defines the max weight value allowed for custom PriorityPolicy
	MaxWeight = MaxTotalScore / MaxCustomPriorityScore
)

func appendPluginSet(dst *PluginSet, src *PluginSet) *PluginSet {
	if dst == nil {
		dst = &PluginSet{}
	}
	if src != nil {
		dst.Enabled = append(dst.Enabled, src.Enabled...)
		dst.Disabled = append(dst.Disabled, src.Disabled...)
	}
	return dst
}

// Append appends src Plugins to current Plugins. If a PluginSet is nil, it will
// be created.
func (p *Plugins) Append(src *Plugins) {
	if p == nil || src == nil {
		return
	}
	p.QueueSort = appendPluginSet(p.QueueSort, src.QueueSort)
	p.PreFilter = appendPluginSet(p.PreFilter, src.PreFilter)
	p.Filter = appendPluginSet(p.Filter, src.Filter)
	p.PostFilter = appendPluginSet(p.PostFilter, src.PostFilter)
	p.PreScore = appendPluginSet(p.PreScore, src.PreScore)
	p.Score = appendPluginSet(p.Score, src.Score)
	p.Reserve = appendPluginSet(p.Reserve, src.Reserve)
	p.Permit = appendPluginSet(p.Permit, src.Permit)
	p.PreBind = appendPluginSet(p.PreBind, src.PreBind)
	p.Bind = appendPluginSet(p.Bind, src.Bind)
	p.PostBind = appendPluginSet(p.PostBind, src.PostBind)
}

// Apply merges the plugin configuration from custom plugins, handling disabled sets.
func (p *Plugins) Apply(customPlugins *Plugins) {
	if customPlugins == nil {
		return
	}

	p.QueueSort = mergePluginSets(p.QueueSort, customPlugins.QueueSort)
	p.PreFilter = mergePluginSets(p.PreFilter, customPlugins.PreFilter)
	p.Filter = mergePluginSets(p.Filter, customPlugins.Filter)
	p.PostFilter = mergePluginSets(p.PostFilter, customPlugins.PostFilter)
	p.PreScore = mergePluginSets(p.PreScore, customPlugins.PreScore)
	p.Score = mergePluginSets(p.Score, customPlugins.Score)
	p.Reserve = mergePluginSets(p.Reserve, customPlugins.Reserve)
	p.Permit = mergePluginSets(p.Permit, customPlugins.Permit)
	p.PreBind = mergePluginSets(p.PreBind, customPlugins.PreBind)
	p.Bind = mergePluginSets(p.Bind, customPlugins.Bind)
	p.PostBind = mergePluginSets(p.PostBind, customPlugins.PostBind)
}

func mergePluginSets(defaultPluginSet, customPluginSet *PluginSet) *PluginSet {
	if customPluginSet == nil {
		customPluginSet = &PluginSet{}
	}

	if defaultPluginSet == nil {
		defaultPluginSet = &PluginSet{}
	}

	disabledPlugins := sets.NewString()
	for _, disabledPlugin := range customPluginSet.Disabled {
		disabledPlugins.Insert(disabledPlugin.Name)
	}

	enabledPlugins := []Plugin{}
	if !disabledPlugins.Has("*") {
		for _, defaultEnabledPlugin := range defaultPluginSet.Enabled {
			if disabledPlugins.Has(defaultEnabledPlugin.Name) {
				continue
			}

			enabledPlugins = append(enabledPlugins, defaultEnabledPlugin)
		}
	}

	enabledPlugins = append(enabledPlugins, customPluginSet.Enabled...)

	return &PluginSet{Enabled: enabledPlugins}
}

// Extender holds the parameters used to communicate with the extender. If a verb is unspecified/empty,
// it is assumed that the extender chose not to provide that extension.
type Extender struct {
	// URLPrefix at which the extender is available
	URLPrefix string
	// Verb for the filter call, empty if not supported. This verb is appended to the URLPrefix when issuing the filter call to extender.
	FilterVerb string
	// Verb for the preempt call, empty if not supported. This verb is appended to the URLPrefix when issuing the preempt call to extender.
	PreemptVerb string
	// Verb for the prioritize call, empty if not supported. This verb is appended to the URLPrefix when issuing the prioritize call to extender.
	PrioritizeVerb string
	// The numeric multiplier for the node scores that the prioritize call generates.
	// The weight should be a positive integer
	Weight int64
	// Verb for the bind call, empty if not supported. This verb is appended to the URLPrefix when issuing the bind call to extender.
	// If this method is implemented by the extender, it is the extender's responsibility to bind the pod to apiserver. Only one extender
	// can implement this function.
	BindVerb string
	// EnableHTTPS specifies whether https should be used to communicate with the extender
	EnableHTTPS bool
	// TLSConfig specifies the transport layer security config
	TLSConfig *ExtenderTLSConfig
	// HTTPTimeout specifies the timeout duration for a call to the extender. Filter timeout fails the scheduling of the pod. Prioritize
	// timeout is ignored, k8s/other extenders priorities are used to select the node.
	HTTPTimeout metav1.Duration
	// NodeCacheCapable specifies that the extender is capable of caching node information,
	// so the scheduler should only send minimal information about the eligible nodes
	// assuming that the extender already cached full details of all nodes in the cluster
	NodeCacheCapable bool
	// ManagedResources is a list of extended resources that are managed by
	// this extender.
	// - A pod will be sent to the extender on the Filter, Prioritize and Bind
	//   (if the extender is the binder) phases iff the pod requests at least
	//   one of the extended resources in this list. If empty or unspecified,
	//   all pods will be sent to this extender.
	// - If IgnoredByScheduler is set to true for a resource, kube-scheduler
	//   will skip checking the resource in predicates.
	// +optional
	ManagedResources []ExtenderManagedResource
	// Ignorable specifies if the extender is ignorable, i.e. scheduling should not
	// fail when the extender returns an error or is not reachable.
	Ignorable bool
}

// ExtenderManagedResource describes the arguments of extended resources
// managed by an extender.
type ExtenderManagedResource struct {
	// Name is the extended resource name.
	Name string
	// IgnoredByScheduler indicates whether kube-scheduler should ignore this
	// resource when applying predicates.
	IgnoredByScheduler bool
}

// ExtenderTLSConfig contains settings to enable TLS with extender
type ExtenderTLSConfig struct {
	// Server should be accessed without verifying the TLS certificate. For testing only.
	Insecure bool
	// ServerName is passed to the server for SNI and is used in the client to check server
	// certificates against. If ServerName is empty, the hostname used to contact the
	// server is used.
	ServerName string

	// Server requires TLS client certificate authentication
	CertFile string
	// Server requires TLS client certificate authentication
	KeyFile string
	// Trusted root certificates for server
	CAFile string

	// CertData holds PEM-encoded bytes (typically read from a client certificate file).
	// CertData takes precedence over CertFile
	CertData []byte
	// KeyData holds PEM-encoded bytes (typically read from a client certificate key file).
	// KeyData takes precedence over KeyFile
	KeyData []byte `datapolicy:"security-key"`
	// CAData holds PEM-encoded bytes (typically read from a root certificates bundle).
	// CAData takes precedence over CAFile
	CAData []byte
}
