/*
Copyright 2017 The Kubernetes Authors.

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

package options

import (
	"github.com/spf13/pflag"
	"k8s.io/component-base/config"
)

// BindLeaderElectionFlags binds the LeaderElectionConfiguration struct fields to a flagset
// BindLeaderElectionFlags 添加多节点领导者选举相关参数
func BindLeaderElectionFlags(l *config.LeaderElectionConfiguration, fs *pflag.FlagSet) {
	// --leader-elect: 用于启用多 kube-scheduler 节点 (集群运行模式) 的选举功能, 被选为领导者的节点负责处理工作,
	// 其他节点处于阻塞状态 (默认值为 true)
	fs.BoolVar(&l.LeaderElect, "leader-elect", l.LeaderElect, ""+
		"Start a leader election client and gain leadership before "+
		"executing the main loop. Enable this when running replicated "+
		"components for high availability.")
	// --leader-elect-lease-duration: 用于设置选举过程中非领导者节点等待选举的时间间隔 (默认值为 15秒)
	fs.DurationVar(&l.LeaseDuration.Duration, "leader-elect-lease-duration", l.LeaseDuration.Duration, ""+
		"The duration that non-leader candidates will wait after observing a leadership "+
		"renewal until attempting to acquire leadership of a led but unrenewed leader "+
		"slot. This is effectively the maximum duration that a leader can be stopped "+
		"before it is replaced by another candidate. This is only applicable if leader "+
		"election is enabled.")
	// --leader-elect-renew-deadline: 用于设置领导者节点重申领导者身份的间隔时间,小于或等于 lease-duration 值 (默认值为 10 秒)
	fs.DurationVar(&l.RenewDeadline.Duration, "leader-elect-renew-deadline", l.RenewDeadline.Duration, ""+
		"The interval between attempts by the acting master to renew a leadership slot "+
		"before it stops leading. This must be less than or equal to the lease duration. "+
		"This is only applicable if leader election is enabled.")
	// --leader-elect-retry-period: 用于设置领导者重新选举的等待间隔 (默认值为 2 秒)
	fs.DurationVar(&l.RetryPeriod.Duration, "leader-elect-retry-period", l.RetryPeriod.Duration, ""+
		"The duration the clients should wait between attempting acquisition and renewal "+
		"of a leadership. This is only applicable if leader election is enabled.")
	// --leader-elect-resource-lock: 用于设置在领导者选举期间用于锁定资源对象的类型 (默认值为 ednpoints)
	fs.StringVar(&l.ResourceLock, "leader-elect-resource-lock", l.ResourceLock, ""+
		"The type of resource object that is used for locking during "+
		"leader election. Supported options are 'endpoints', 'configmaps', "+
		"'leases', 'endpointsleases' and 'configmapsleases'.")
	// --leader-elect-resource-name: 用于设置在领导者选举期间用于锁定资源对象的名称
	fs.StringVar(&l.ResourceName, "leader-elect-resource-name", l.ResourceName, ""+
		"The name of resource object that is used for locking during "+
		"leader election.")
	// --leader-elect-resource-namespace: 用于设置在领导者选举期间用于锁定资源对象的命名空间
	fs.StringVar(&l.ResourceNamespace, "leader-elect-resource-namespace", l.ResourceNamespace, ""+
		"The namespace of resource object that is used for locking during "+
		"leader election.")
}
