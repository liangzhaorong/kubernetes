/*
Copyright 2019 The Kubernetes Authors.

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

package nodeports

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

// NodePorts is a plugin that checks if a node has free ports for the requested pod ports.
// NodePorts 插件用于检查当前节点上是否存在待调度的 Pod 所请求的空闲端口
type NodePorts struct{}

var _ framework.PreFilterPlugin = &NodePorts{}
var _ framework.FilterPlugin = &NodePorts{}

const (
	// Name is the name of the plugin used in the plugin registry and configurations.
	Name = "NodePorts"

	// preFilterStateKey is the key in CycleState to NodePorts pre-computed data.
	// Using the name of the plugin will likely help us avoid collisions with other plugins.
	preFilterStateKey = "PreFilter" + Name

	// ErrReason when node ports aren't available.
	ErrReason = "node(s) didn't have free ports for the requested pod ports"
)

type preFilterState []*v1.ContainerPort

// Clone the prefilter state.
func (s preFilterState) Clone() framework.StateData {
	// The state is not impacted by adding/removing existing pods, hence we don't need to make a deep copy.
	return s
}

// Name returns name of the plugin. It is used in logs, etc.
func (pl *NodePorts) Name() string {
	return Name
}

// getContainerPorts returns the used host ports of Pods: if 'port' was used, a 'port:true' pair
// will be in the result; but it does not resolve port conflict.
func getContainerPorts(pods ...*v1.Pod) []*v1.ContainerPort {
	ports := []*v1.ContainerPort{}
	for _, pod := range pods {
		for j := range pod.Spec.Containers {
			container := &pod.Spec.Containers[j]
			for k := range container.Ports {
				ports = append(ports, &container.Ports[k])
			}
		}
	}
	return ports
}

// PreFilter invoked at the prefilter extension point.
func (pl *NodePorts) PreFilter(ctx context.Context, cycleState *framework.CycleState, pod *v1.Pod) *framework.Status {
	// 获取该 pod 配置的所有容器端口对象 ContainerPort
	s := getContainerPorts(pod)
	// 将该 pod 的端口信息以 preFilterStateKey 为 key 存储到框架的 CycleState 中, 以便后续阶段的插件使用
	cycleState.Write(preFilterStateKey, preFilterState(s))
	return nil
}

// PreFilterExtensions do not exist for this plugin.
func (pl *NodePorts) PreFilterExtensions() framework.PreFilterExtensions {
	return nil
}

func getPreFilterState(cycleState *framework.CycleState) (preFilterState, error) {
	// 从 CycleState 中获取该 key 对应的数据, 即 PreFilter 阶段保存的该 pod 的容器端口信息
	c, err := cycleState.Read(preFilterStateKey)
	if err != nil {
		// preFilterState doesn't exist, likely PreFilter wasn't invoked.
		return nil, fmt.Errorf("reading %q from cycleState: %w", preFilterStateKey, err)
	}

	s, ok := c.(preFilterState)
	if !ok {
		return nil, fmt.Errorf("%+v  convert to nodeports.preFilterState error", c)
	}
	return s, nil
}

// Filter invoked at the filter extension point.
func (pl *NodePorts) Filter(ctx context.Context, cycleState *framework.CycleState, pod *v1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
	// 从 CycleState 中获取 PreFilter 阶段缓存的该 pod 的容器端口信息.
	wantPorts, err := getPreFilterState(cycleState)
	if err != nil {
		return framework.AsStatus(err)
	}

	// 检测该节点上已经分配了的端口与该 pod 请求的容器端口是否有冲突
	fits := fitsPorts(wantPorts, nodeInfo)
	if !fits {
		return framework.NewStatus(framework.Unschedulable, ErrReason)
	}

	return nil
}

// Fits checks if the pod fits the node.
func Fits(pod *v1.Pod, nodeInfo *framework.NodeInfo) bool {
	return fitsPorts(getContainerPorts(pod), nodeInfo)
}

// fitsPorts 检测该节点上已经分配了的端口与该 pod 请求的容器端口是否有冲突, 返回 false 表示有冲突, 不可调度到该节点上.
func fitsPorts(wantPorts []*v1.ContainerPort, nodeInfo *framework.NodeInfo) bool {
	// try to see whether existingPorts and wantPorts will conflict or not
	existingPorts := nodeInfo.UsedPorts
	for _, cp := range wantPorts {
		if existingPorts.CheckConflict(cp.HostIP, string(cp.Protocol), cp.HostPort) {
			return false
		}
	}
	return true
}

// New initializes a new plugin and returns it.
func New(_ runtime.Object, _ framework.Handle) (framework.Plugin, error) {
	return &NodePorts{}, nil
}
