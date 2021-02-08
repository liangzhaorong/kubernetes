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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const EndpointsResourceLock = "endpoints"

// LeaderElectionConfiguration defines the configuration of leader election
// clients for components that can run with leader election enabled.
type LeaderElectionConfiguration struct {
	// leaderElect enables a leader election client to gain leadership
	// before executing the main loop. Enable this when running replicated
	// components for high availability.
	LeaderElect *bool `json:"leaderElect"`
	// leaseDuration is the duration that non-leader candidates will wait
	// after observing a leadership renewal until attempting to acquire
	// leadership of a led but unrenewed leader slot. This is effectively the
	// maximum duration that a leader can be stopped before it is replaced
	// by another candidate. This is only applicable if leader election is
	// enabled.
	LeaseDuration metav1.Duration `json:"leaseDuration"`
	// renewDeadline is the interval between attempts by the acting master to
	// renew a leadership slot before it stops leading. This must be less
	// than or equal to the lease duration. This is only applicable if leader
	// election is enabled.
	RenewDeadline metav1.Duration `json:"renewDeadline"`
	// retryPeriod is the duration the clients should wait between attempting
	// acquisition and renewal of a leadership. This is only applicable if
	// leader election is enabled.
	RetryPeriod metav1.Duration `json:"retryPeriod"`
	// resourceLock indicates the resource object type that will be used to lock
	// during leader election cycles.
	ResourceLock string `json:"resourceLock"`
	// resourceName indicates the name of resource object that will be used to lock
	// during leader election cycles.
	ResourceName string `json:"resourceName"`
	// resourceName indicates the namespace of resource object that will be used to lock
	// during leader election cycles.
	ResourceNamespace string `json:"resourceNamespace"`
}

// DebuggingConfiguration holds configuration for Debugging related features.
//
// DebuggingConfiguration 保存用于调试相关功能的配置.
type DebuggingConfiguration struct {
	// enableProfiling enables profiling via web interface host:port/debug/pprof/
	//
	// enableProfiling 通过 web 接口 host:port/debug/pprof/ 来启用 profiling.
	EnableProfiling *bool `json:"enableProfiling,omitempty"`
	// enableContentionProfiling enables lock contention profiling, if
	// enableProfiling is true.
	//
	// EnableContentionProfiling 如果 EnableProfiling 为 true, 则 enableContentionProfiling 启用锁竞争 profiling.
	EnableContentionProfiling *bool `json:"enableContentionProfiling,omitempty"`
}

// ClientConnectionConfiguration contains details for constructing a client.
type ClientConnectionConfiguration struct {
	// kubeconfig is the path to a KubeConfig file.
	Kubeconfig string `json:"kubeconfig"`
	// acceptContentTypes defines the Accept header sent by clients when connecting to a server, overriding the
	// default value of 'application/json'. This field will control all connections to the server used by a particular
	// client.
	AcceptContentTypes string `json:"acceptContentTypes"`
	// contentType is the content type used when sending data to the server from this client.
	ContentType string `json:"contentType"`
	// qps controls the number of queries per second allowed for this connection.
	QPS float32 `json:"qps"`
	// burst allows extra queries to accumulate when a client is exceeding its rate.
	Burst int32 `json:"burst"`
}

// LoggingConfiguration contains logging options
// Refer [Logs Options](https://github.com/kubernetes/component-base/blob/master/logs/options.go) for more information.
type LoggingConfiguration struct {
	// Format Flag specifies the structure of log messages.
	// default value of format is `text`
	Format string `json:"format,omitempty"`
	// [Experimental] When enabled prevents logging of fields tagged as sensitive (passwords, keys, tokens).
	// Runtime log sanitization may introduce significant computation overhead and therefore should not be enabled in production.`)
	Sanitization bool `json:"sanitization,omitempty"`
}
