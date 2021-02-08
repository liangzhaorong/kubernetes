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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClientConnectionConfiguration contains details for constructing a client.
type ClientConnectionConfiguration struct {
	// kubeconfig is the path to a KubeConfig file.
	Kubeconfig string
	// acceptContentTypes defines the Accept header sent by clients when connecting to a server, overriding the
	// default value of 'application/json'. This field will control all connections to the server used by a particular
	// client.
	AcceptContentTypes string
	// contentType is the content type used when sending data to the server from this client.
	ContentType string
	// qps controls the number of queries per second allowed for this connection.
	QPS float32
	// burst allows extra queries to accumulate when a client is exceeding its rate.
	Burst int32
}

// LeaderElectionConfiguration defines the configuration of leader election
// clients for components that can run with leader election enabled.
type LeaderElectionConfiguration struct {
	// leaderElect enables a leader election client to gain leadership
	// before executing the main loop. Enable this when running replicated
	// components for high availability.
	//
	// --leader-elect: 用于启用多 kube-scheduler 节点 (集群运行模式) 的选举功能, 被选为领导者的节点负责处理工作,
	// 其他节点处于阻塞状态 (默认值为 true)
	LeaderElect bool
	// leaseDuration is the duration that non-leader candidates will wait
	// after observing a leadership renewal until attempting to acquire
	// leadership of a led but unrenewed leader slot. This is effectively the
	// maximum duration that a leader can be stopped before it is replaced
	// by another candidate. This is only applicable if leader election is
	// enabled.
	//
	// --leader-elect-lease-duration: 用于设置选举过程中非领导者节点等待选举的时间间隔 (默认值为 15秒)
	LeaseDuration metav1.Duration
	// renewDeadline is the interval between attempts by the acting master to
	// renew a leadership slot before it stops leading. This must be less
	// than or equal to the lease duration. This is only applicable if leader
	// election is enabled.
	//
	// --leader-elect-renew-deadline: 用于设置领导者节点重申领导者身份的间隔时间,小于或等于 lease-duration 值 (默认值为 10 秒)
	RenewDeadline metav1.Duration
	// retryPeriod is the duration the clients should wait between attempting
	// acquisition and renewal of a leadership. This is only applicable if
	// leader election is enabled.
	//
	// --leader-elect-retry-period: 用于设置领导者重新选举的等待间隔 (默认值为 2 秒)
	RetryPeriod metav1.Duration
	// resourceLock indicates the resource object type that will be used to lock
	// during leader election cycles.
	//
	// --leader-elect-resource-lock: 用于设置在领导者选举期间用于锁定资源对象的类型 (默认值为 ednpoints)
	ResourceLock string
	// resourceName indicates the name of resource object that will be used to lock
	// during leader election cycles.
	//
	// --leader-elect-resource-name: 用于设置在领导者选举期间用于锁定资源对象的名称
	ResourceName string
	// resourceName indicates the namespace of resource object that will be used to lock
	// during leader election cycles.
	//
	// --leader-elect-resource-namespace: 用于设置在领导者选举期间用于锁定资源对象的命名空间
	ResourceNamespace string
}

// DebuggingConfiguration holds configuration for Debugging related features.
type DebuggingConfiguration struct {
	// enableProfiling enables profiling via web interface host:port/debug/pprof/
	EnableProfiling bool
	// enableContentionProfiling enables lock contention profiling, if
	// enableProfiling is true.
	EnableContentionProfiling bool
}

// LoggingConfiguration contains logging options
// Refer [Logs Options](https://github.com/kubernetes/component-base/blob/master/logs/options.go) for more information.
type LoggingConfiguration struct {
	// Format Flag specifies the structure of log messages.
	// default value of format is `text`
	Format string
	// [Experimental] When enabled prevents logging of fields tagged as sensitive (passwords, keys, tokens).
	// Runtime log sanitization may introduce significant computation overhead and therefore should not be enabled in production.`)
	Sanitization bool
}
