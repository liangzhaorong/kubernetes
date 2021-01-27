/*
Copyright 2016 The Kubernetes Authors.

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

package podnodeselector

import (
	"context"
	"fmt"
	"io"
	"reflect"

	"k8s.io/klog/v2"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/apiserver/pkg/admission"
	genericadmissioninitializer "k8s.io/apiserver/pkg/admission/initializer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	api "k8s.io/kubernetes/pkg/apis/core"
)

// NamespaceNodeSelectors is for assigning node selectors labels to
// namespaces. Default value is the annotation key
// scheduler.alpha.kubernetes.io/node-selector
var NamespaceNodeSelectors = []string{"scheduler.alpha.kubernetes.io/node-selector"}

// PluginName is a string with the name of the plugin
const PluginName = "PodNodeSelector"

// Register registers a plugin
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		// TODO move this to a versioned configuration file format.
		pluginConfig := readConfig(config)
		plugin := NewPodNodeSelector(pluginConfig.PodNodeSelectorPluginConfig)
		return plugin, nil
	})
}

// PodNodeSelector 准入控制器通过读取命名空间注释和准入控制器配置文件来限制 Pod 资源对象可在
// 命名空间内使用的节点选择器. PodNodeSelector 准入控制器会对拦截的 kube-apiserver 请求中的
// Pod 资源对象进行修改, 将节点选择器与 Pod 资源对象的节点选择器进行合并并赋值给 Pod 资源对象
// 的节点选择器(即 pod.Spec.NodeSelector).
//
// Pod 资源对象的节点选择器 (即 pod.Spec.NodeSelector) 必须为 true, 才能使 Pod 资源对象选择
// 到合适的节点.

// Plugin is an implementation of admission.Interface.
type Plugin struct {
	*admission.Handler
	client          kubernetes.Interface
	namespaceLister corev1listers.NamespaceLister
	// global default node selector and namespace whitelists in a cluster.
	clusterNodeSelectors map[string]string
}

var _ = genericadmissioninitializer.WantsExternalKubeClientSet(&Plugin{})
var _ = genericadmissioninitializer.WantsExternalKubeInformerFactory(&Plugin{})

type pluginConfig struct {
	PodNodeSelectorPluginConfig map[string]string
}

// readConfig reads default value of clusterDefaultNodeSelector
// from the file provided with --admission-control-config-file
// If the file is not supplied, it defaults to ""
// The format in a file:
// podNodeSelectorPluginConfig:
//  clusterDefaultNodeSelector: <node-selectors-labels>
//  namespace1: <node-selectors-labels>
//  namespace2: <node-selectors-labels>
func readConfig(config io.Reader) *pluginConfig {
	defaultConfig := &pluginConfig{}
	if config == nil || reflect.ValueOf(config).IsNil() {
		return defaultConfig
	}
	d := yaml.NewYAMLOrJSONDecoder(config, 4096)
	for {
		if err := d.Decode(defaultConfig); err != nil {
			if err != io.EOF {
				continue
			}
		}
		break
	}
	return defaultConfig
}

// Admit enforces that pod and its namespace node label selectors matches at least a node in the cluster.
func (p *Plugin) Admit(ctx context.Context, a admission.Attributes, o admission.ObjectInterfaces) error {
	// 忽略 Pod 以外的资源
	if shouldIgnore(a) {
		return nil
	}
	// 判断该准入控制器是否已完成初始化
	if !p.WaitForReady() {
		return admission.NewForbidden(a, fmt.Errorf("not yet ready to handle request"))
	}

	resource := a.GetResource().GroupResource()
	pod := a.GetObject().(*api.Pod)
	// 通过 p.getNamespaceNodeSelectorMap 函数选择节点选择器 (namespaceNodeSelector), 它是通过读取
	// 命名空间注释和配置文件来选择节点选择器的.
	namespaceNodeSelector, err := p.getNamespaceNodeSelectorMap(a.GetNamespace())
	if err != nil {
		return err
	}

	// 通过 labels.Conflicts 判断节点选择器与资源对象的节点选择器是否存在冲突. 如果存在冲突, 则通过
	// errors.NewForbidden 返回 HTTP 403 Forbidden;
	if labels.Conflicts(namespaceNodeSelector, labels.Set(pod.Spec.NodeSelector)) {
		return errors.NewForbidden(resource, pod.Name, fmt.Errorf("pod node label selector conflicts with its namespace node label selector"))
	}

	// 如果不存在冲突, 则通过 labels.Merge 将节点选择器与资源对象的节点选择器进行合并并赋值给
	// Pod 资源对象的节点选择器(即 pod.Spec.NodeSelector).
	// Merge pod node selector = namespace node selector + current pod node selector
	// second selector wins
	podNodeSelectorLabels := labels.Merge(namespaceNodeSelector, pod.Spec.NodeSelector)
	pod.Spec.NodeSelector = map[string]string(podNodeSelectorLabels)
	return p.Validate(ctx, a, o)
}

// Validate ensures that the pod node selector is allowed
// Validate 执行完 PodNodeSelector 准入控制器变更操作 Admit 以后执行该 Validate 验证操作.
func (p *Plugin) Validate(ctx context.Context, a admission.Attributes, o admission.ObjectInterfaces) error {
	if shouldIgnore(a) {
		return nil
	}
	if !p.WaitForReady() {
		return admission.NewForbidden(a, fmt.Errorf("not yet ready to handle request"))
	}

	resource := a.GetResource().GroupResource()
	pod := a.GetObject().(*api.Pod)

	namespaceNodeSelector, err := p.getNamespaceNodeSelectorMap(a.GetNamespace())
	if err != nil {
		return err
	}
	// 验证 pod.Spec.NodeSelector 资源对象的节点选择器是否与准入控制器配置文件中定义的节点选择器存在冲突.
	if labels.Conflicts(namespaceNodeSelector, labels.Set(pod.Spec.NodeSelector)) {
		return errors.NewForbidden(resource, pod.Name, fmt.Errorf("pod node label selector conflicts with its namespace node label selector"))
	}

	// whitelist verification
	whitelist, err := labels.ConvertSelectorToLabelsMap(p.clusterNodeSelectors[a.GetNamespace()])
	if err != nil {
		return err
	}
	if !isSubset(pod.Spec.NodeSelector, whitelist) {
		return errors.NewForbidden(resource, pod.Name, fmt.Errorf("pod node label selector labels conflict with its namespace whitelist"))
	}

	return nil
}

// getNamespaceNodeSelectorMap 获取节点选择器 (namespaceNodeSelector), 它是通过读取命名空间注释和配置文件来选择节点选择器的.
func (p *Plugin) getNamespaceNodeSelectorMap(namespaceName string) (labels.Set, error) {
	namespace, err := p.namespaceLister.Get(namespaceName)
	if errors.IsNotFound(err) {
		namespace, err = p.defaultGetNamespace(namespaceName)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, err
			}
			return nil, errors.NewInternalError(err)
		}
	} else if err != nil {
		return nil, errors.NewInternalError(err)
	}

	return p.getNodeSelectorMap(namespace)
}

func shouldIgnore(a admission.Attributes) bool {
	resource := a.GetResource().GroupResource()
	if resource != api.Resource("pods") {
		return true
	}
	if a.GetSubresource() != "" {
		// only run the checks below on pods proper and not subresources
		return true
	}

	_, ok := a.GetObject().(*api.Pod)
	if !ok {
		klog.Errorf("expected pod but got %s", a.GetKind().Kind)
		return true
	}

	return false
}

// NewPodNodeSelector initializes a podNodeSelector
// NewPodNodeSelector 初始化一个 podNodeSelector 准入控制器实例
func NewPodNodeSelector(clusterNodeSelectors map[string]string) *Plugin {
	return &Plugin{
		Handler:              admission.NewHandler(admission.Create),
		clusterNodeSelectors: clusterNodeSelectors,
	}
}

// SetExternalKubeClientSet sets the plugin's client
func (p *Plugin) SetExternalKubeClientSet(client kubernetes.Interface) {
	p.client = client
}

// SetExternalKubeInformerFactory configures the plugin's informer factory
func (p *Plugin) SetExternalKubeInformerFactory(f informers.SharedInformerFactory) {
	namespaceInformer := f.Core().V1().Namespaces()
	p.namespaceLister = namespaceInformer.Lister()
	p.SetReadyFunc(namespaceInformer.Informer().HasSynced)
}

// ValidateInitialization verifies the object has been properly initialized
func (p *Plugin) ValidateInitialization() error {
	if p.namespaceLister == nil {
		return fmt.Errorf("missing namespaceLister")
	}
	if p.client == nil {
		return fmt.Errorf("missing client")
	}
	return nil
}

func (p *Plugin) defaultGetNamespace(name string) (*corev1.Namespace, error) {
	namespace, err := p.client.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("namespace %s does not exist", name)
	}
	return namespace, nil
}

func (p *Plugin) getNodeSelectorMap(namespace *corev1.Namespace) (labels.Set, error) {
	selector := labels.Set{}
	var err error
	found := false
	if len(namespace.ObjectMeta.Annotations) > 0 {
		for _, annotation := range NamespaceNodeSelectors {
			// 如果命名空间中具有带键的注释(即 "scheduler.alpha.kubernetes.io/node-selector"),
			// 则将其值用作节点选择器
			if ns, ok := namespace.ObjectMeta.Annotations[annotation]; ok {
				labelsMap, err := labels.ConvertSelectorToLabelsMap(ns)
				if err != nil {
					return labels.Set{}, err
				}

				if labels.Conflicts(selector, labelsMap) {
					nsName := namespace.ObjectMeta.Name
					return labels.Set{}, fmt.Errorf("%s annotations' node label selectors conflict", nsName)
				}
				selector = labels.Merge(selector, labelsMap)
				found = true
			}
		}
	}
	if !found {
		// 如果命名空间中没有这样的注释 (即 "scheduler.alpha.kubernetes.io/node-selector"), 则使用准入控制器
		// 配置文件中定义的 clusterDefaultNodeSelector 作为节点选择器.
		selector, err = labels.ConvertSelectorToLabelsMap(p.clusterNodeSelectors["clusterDefaultNodeSelector"])
		if err != nil {
			return labels.Set{}, err
		}
	}
	return selector, nil
}

func isSubset(subSet, superSet labels.Set) bool {
	if len(superSet) == 0 {
		return true
	}

	for k, v := range subSet {
		value, ok := superSet[k]
		if !ok {
			return false
		}
		if value != v {
			return false
		}
	}
	return true
}
