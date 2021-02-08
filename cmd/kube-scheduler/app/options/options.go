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

package options

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	apiserveroptions "k8s.io/apiserver/pkg/server/options"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/events"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	cliflag "k8s.io/component-base/cli/flag"
	componentbaseconfig "k8s.io/component-base/config"
	"k8s.io/component-base/config/options"
	configv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/component-base/logs"
	"k8s.io/component-base/metrics"
	"k8s.io/klog/v2"
	kubeschedulerconfigv1beta1 "k8s.io/kube-scheduler/config/v1beta1"
	schedulerappconfig "k8s.io/kubernetes/cmd/kube-scheduler/app/config"
	"k8s.io/kubernetes/pkg/scheduler"
	kubeschedulerconfig "k8s.io/kubernetes/pkg/scheduler/apis/config"
	kubeschedulerscheme "k8s.io/kubernetes/pkg/scheduler/apis/config/scheme"
	"k8s.io/kubernetes/pkg/scheduler/apis/config/validation"
)

// Options has all the params needed to run a Scheduler
//
// Options 是保存运行 Scheduler 所需的所有的参数.
type Options struct {
	// The default values. These are overridden if ConfigFile is set or by values in InsecureServing.
	// 调度策略相关参数. 有默认值. 如果设置了 ConfigFile 或 InsecureServing, 这些默认值值将被覆盖.
	ComponentConfig kubeschedulerconfig.KubeSchedulerConfiguration

	// HTTPS 服务相关参数
	SecureServing           *apiserveroptions.SecureServingOptionsWithLoopback
	// HTTP 服务相关参数
	CombinedInsecureServing *CombinedInsecureServingOptions
	// 认证相关参数
	Authentication          *apiserveroptions.DelegatingAuthenticationOptions
	// 授权相关参数
	Authorization           *apiserveroptions.DelegatingAuthorizationOptions
	// 指标监控相关参数
	Metrics                 *metrics.Options
	Logs                    *logs.Options
	// 旧的调度策略相关的参数, 已标记为过期并会在将来的版本中去掉.
	Deprecated              *DeprecatedOptions

	// ConfigFile is the location of the scheduler server's configuration file.
	//
	// ConfigFile 指定 kube-scheduler 的配置文件的路径. 由 --config 参数配置
	// --config: 用于设置 kube-scheduler 配置文件路径
	ConfigFile string

	// WriteConfigTo is the path where the default configuration will be written.
	//
	// WriteConfigTo 是将写入默认配置的路径. 该字段由 --write-config-to 参数配置.
	// --write-config-to: 如果指定该参数, 将配置参数写入 kubeconfig 文件并退出
	WriteConfigTo string

	// 由 --master 参数配置.
	// --master: 用于设置 Kubernetes API Server 地址, 该参数会覆盖 kubeconfig 中的相同参数
	Master string
}

// NewOptions returns default scheduler app options.
//
// NewOptions 返回默认的 scheduler app 选项 Options 对象
func NewOptions() (*Options, error) {
	// 创建 KubeSchedulerConfiguration 结构体, 即关于调度策略相关的参数, 该返回对象已设置了默认值
	cfg, err := newDefaultComponentConfig()
	if err != nil {
		return nil, err
	}

	hhost, hport, err := splitHostIntPort(cfg.HealthzBindAddress)
	if err != nil {
		return nil, err
	}

	o := &Options{
		ComponentConfig: *cfg,
		SecureServing:   apiserveroptions.NewSecureServingOptions().WithLoopback(),
		CombinedInsecureServing: &CombinedInsecureServingOptions{
			Healthz: (&apiserveroptions.DeprecatedInsecureServingOptions{
				BindNetwork: "tcp",
			}).WithLoopback(),
			Metrics: (&apiserveroptions.DeprecatedInsecureServingOptions{
				BindNetwork: "tcp",
			}).WithLoopback(),
			BindPort:    hport,
			BindAddress: hhost,
		},
		Authentication: apiserveroptions.NewDelegatingAuthenticationOptions(),
		Authorization:  apiserveroptions.NewDelegatingAuthorizationOptions(),
		Deprecated: &DeprecatedOptions{
			UseLegacyPolicyConfig:          false,
			PolicyConfigMapNamespace:       metav1.NamespaceSystem,
			SchedulerName:                  corev1.DefaultSchedulerName,
			HardPodAffinitySymmetricWeight: 1,
		},
		Metrics: metrics.NewOptions(),
		Logs:    logs.NewOptions(),
	}

	o.Authentication.TolerateInClusterLookupFailure = true
	o.Authentication.RemoteKubeConfigFileOptional = true
	o.Authorization.RemoteKubeConfigFileOptional = true
	// 指定不对 /healthz 前缀的路径进行授权.
	o.Authorization.AlwaysAllowPaths = []string{"/healthz"}

	// Set the PairName but leave certificate directory blank to generate in-memory by default
	//
	// 设置 PairName 的值, 但默认情况下将证书目录置为空字符串以生成内存.
	o.SecureServing.ServerCert.CertDirectory = ""
	o.SecureServing.ServerCert.PairName = "kube-scheduler"
	// 设置 kube-scheduler 的 HTTPS 服务默认绑定 10259 端口
	o.SecureServing.BindPort = kubeschedulerconfig.DefaultKubeSchedulerPort

	return o, nil
}

func splitHostIntPort(s string) (string, int, error) {
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		return "", 0, err
	}
	portInt, err := strconv.Atoi(port)
	if err != nil {
		return "", 0, err
	}
	return host, portInt, err
}

// newDefaultComponentConfig 创建 kubeschedulerconfig.KubeSchedulerConfiguration 对象, 并且该对象已被设置了默认值
// 这个对象代表 kube-scheduler 调度策略相关的参数.
func newDefaultComponentConfig() (*kubeschedulerconfig.KubeSchedulerConfiguration, error) {
	// 实例化 v1beta1 版本的 KubeSchedulerConfiguration 对象实例
	versionedCfg := kubeschedulerconfigv1beta1.KubeSchedulerConfiguration{}
	versionedCfg.DebuggingConfiguration = *configv1alpha1.NewRecommendedDebuggingConfiguration()

	// 调用 scheme 缓存的 versionedCfg 对象对应的资源类型的默认值函数来设置 versionedCfg 的默认值
	kubeschedulerscheme.Scheme.Default(&versionedCfg)
	// 实例化内部版本的 KubeSchedulerConfiguration 对象
	cfg := kubeschedulerconfig.KubeSchedulerConfiguration{}
	// 将设置有默认值的 v1beta1 版本的 KubeSchedulerConfiguration 转换为内部版本的 KubeSchedulerConfiguration
	if err := kubeschedulerscheme.Scheme.Convert(&versionedCfg, &cfg, nil); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Flags returns flags for a specific scheduler by section name
//
// Flags 按 section 名称返回特定的调度器参数 flags
func (o *Options) Flags() (nfs cliflag.NamedFlagSets) {
	// 该函数按 section 将 kube-scheduler 的命令行参数划分了如下几类.

	// misc flags: 其他参数
	fs := nfs.FlagSet("misc")
	// --config: 用于设置 kube-scheduler 配置文件路径
	fs.StringVar(&o.ConfigFile, "config", o.ConfigFile, `The path to the configuration file. The following flags can overwrite fields in this file:
  --address
  --port
  --use-legacy-policy-config
  --policy-configmap
  --policy-config-file
  --algorithm-provider`)
	// --write-config-to: 如果指定该参数, 将配置参数写入 kubeconfig 文件并退出
	fs.StringVar(&o.WriteConfigTo, "write-config-to", o.WriteConfigTo, "If set, write the configuration values to this file and exit.")
	// --master: 用于设置 Kubernetes API Server 地址, 该参数会覆盖 kubeconfig 中的相同参数
	fs.StringVar(&o.Master, "master", o.Master, "The address of the Kubernetes API server (overrides any value in kubeconfig)")

	// secure serving flags: HTTPS 服务相关参数
	o.SecureServing.AddFlags(nfs.FlagSet("secure serving"))
	// insecure serving flags: HTTP 服务相关参数
	o.CombinedInsecureServing.AddFlags(nfs.FlagSet("insecure serving"))
	// authentication flags: 认证相关参数
	o.Authentication.AddFlags(nfs.FlagSet("authentication"))
	// authorization flags: 授权相关参数
	o.Authorization.AddFlags(nfs.FlagSet("authorization"))
	// deprecated flags: 旧的调度策略相关参数, 已标记为过期并在将来的版本中移除
	o.Deprecated.AddFlags(nfs.FlagSet("deprecated"), &o.ComponentConfig)

	// leader election flags: 多节点领导者选举相关参数
	options.BindLeaderElectionFlags(&o.ComponentConfig.LeaderElection, nfs.FlagSet("leader election"))
	// feature gate flags: 实验性功能相关参数
	utilfeature.DefaultMutableFeatureGate.AddFlag(nfs.FlagSet("feature gate"))
	// metrics flags: 监控指标相关参数
	o.Metrics.AddFlags(nfs.FlagSet("metrics"))
	// logs flags: 日志相关参数
	o.Logs.AddFlags(nfs.FlagSet("logs"))

	return nfs
}

// ApplyTo applies the scheduler options to the given scheduler app configuration.
func (o *Options) ApplyTo(c *schedulerappconfig.Config) error {
	// 若没有配置 --config 参数
	if len(o.ConfigFile) == 0 {
		c.ComponentConfig = o.ComponentConfig

		// apply deprecated flags if no config file is loaded (this is the old behaviour).
		// 若没有指定配置文件, 则将使用旧的、将要废弃的调度器策略
		o.Deprecated.ApplyTo(&c.ComponentConfig)
		// 使用不安全的 HTTP 服务相关参数配置
		if err := o.CombinedInsecureServing.ApplyTo(c, &c.ComponentConfig); err != nil {
			return err
		}
	} else {
		// 从 kube-scheduler 的配置文件中加载调度器配置
		cfg, err := loadConfigFromFile(o.ConfigFile)
		if err != nil {
			return err
		}
		// 验证调度器配置的有效性
		if err := validation.ValidateKubeSchedulerConfiguration(cfg).ToAggregate(); err != nil {
			return err
		}

		// 配置正确, 则将其存到 schedulerappconfig.Config 中的 ComponentConfig 中.
		c.ComponentConfig = *cfg

		// apply any deprecated Policy flags, if applicable
		o.Deprecated.ApplyAlgorithmSourceTo(&c.ComponentConfig)

		// if the user has set CC profiles and is trying to use a Policy config, error out
		// these configs are no longer merged and they should not be used simultaneously
		if !emptySchedulerProfileConfig(c.ComponentConfig.Profiles) && c.ComponentConfig.AlgorithmSource.Policy != nil {
			return fmt.Errorf("cannot set a Plugin config and Policy config")
		}

		// use the loaded config file only, with the exception of --address and --port.
		if err := o.CombinedInsecureServing.ApplyToFromLoadedConfig(c, &c.ComponentConfig); err != nil {
			return err
		}
	}

	// 应用 HTTPS 服务相关参数
	if err := o.SecureServing.ApplyTo(&c.SecureServing, &c.LoopbackClientConfig); err != nil {
		return err
	}
	if o.SecureServing != nil && (o.SecureServing.BindPort != 0 || o.SecureServing.Listener != nil) {
		// 应用认证相关参数
		if err := o.Authentication.ApplyTo(&c.Authentication, c.SecureServing, nil); err != nil {
			return err
		}
		// 应用授权相关参数
		if err := o.Authorization.ApplyTo(&c.Authorization); err != nil {
			return err
		}
	}
	// 应用监控指标相关参数
	o.Metrics.Apply()
	// 应用日志相关参数
	o.Logs.Apply()
	return nil
}

// emptySchedulerProfileConfig returns true if the list of profiles passed to it contains only
// the "default-scheduler" profile with no plugins or pluginconfigs registered
// (this is the default empty profile initialized by defaults.go)
func emptySchedulerProfileConfig(profiles []kubeschedulerconfig.KubeSchedulerProfile) bool {
	return len(profiles) == 1 &&
		len(profiles[0].PluginConfig) == 0 &&
		profiles[0].Plugins == nil
}

// Validate validates all the required options.
// Validate 验证所有配置参数的合法性
func (o *Options) Validate() []error {
	var errs []error

	if err := validation.ValidateKubeSchedulerConfiguration(&o.ComponentConfig).ToAggregate(); err != nil {
		errs = append(errs, err.Errors()...)
	}
	errs = append(errs, o.SecureServing.Validate()...)
	errs = append(errs, o.CombinedInsecureServing.Validate()...)
	errs = append(errs, o.Authentication.Validate()...)
	errs = append(errs, o.Authorization.Validate()...)
	errs = append(errs, o.Deprecated.Validate()...)
	errs = append(errs, o.Metrics.Validate()...)
	errs = append(errs, o.Logs.Validate()...)

	return errs
}

// Config return a scheduler config object
func (o *Options) Config() (*schedulerappconfig.Config, error) {
	// 若配置了 HTTPS 服务相关参数
	if o.SecureServing != nil {
		if err := o.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, []net.IP{net.ParseIP("127.0.0.1")}); err != nil {
			return nil, fmt.Errorf("error creating self-signed certificates: %v", err)
		}
	}

	c := &schedulerappconfig.Config{}
	// 调用 o.ApplyTo 函数, 执行 Options 对象到 schedulerappconfig.Config 对象的转换.
	if err := o.ApplyTo(c); err != nil {
		return nil, err
	}

	// Prepare kube clients.
	client, leaderElectionClient, eventClient, err := createClients(c.ComponentConfig.ClientConnection, o.Master, c.ComponentConfig.LeaderElection.RenewDeadline.Duration)
	if err != nil {
		return nil, err
	}

	// 创建事件管理器
	c.EventBroadcaster = events.NewEventBroadcasterAdapter(eventClient)

	// Set up leader election if enabled.
	var leaderElectionConfig *leaderelection.LeaderElectionConfig
	if c.ComponentConfig.LeaderElection.LeaderElect {
		// Use the scheduler name in the first profile to record leader election.
		coreRecorder := c.EventBroadcaster.DeprecatedNewLegacyRecorder(c.ComponentConfig.Profiles[0].SchedulerName)
		leaderElectionConfig, err = makeLeaderElectionConfig(c.ComponentConfig.LeaderElection, leaderElectionClient, coreRecorder)
		if err != nil {
			return nil, err
		}
	}

	c.Client = client
	c.InformerFactory = scheduler.NewInformerFactory(client, 0)
	c.LeaderElection = leaderElectionConfig

	return c, nil
}

// makeLeaderElectionConfig builds a leader election configuration. It will
// create a new resource lock associated with the configuration.
func makeLeaderElectionConfig(config componentbaseconfig.LeaderElectionConfiguration, client clientset.Interface, recorder record.EventRecorder) (*leaderelection.LeaderElectionConfig, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("unable to get hostname: %v", err)
	}
	// add a uniquifier so that two processes on the same host don't accidentally both become active
	id := hostname + "_" + string(uuid.NewUUID())

	rl, err := resourcelock.New(config.ResourceLock,
		config.ResourceNamespace,
		config.ResourceName,
		client.CoreV1(),
		client.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity:      id,
			EventRecorder: recorder,
		})
	if err != nil {
		return nil, fmt.Errorf("couldn't create resource lock: %v", err)
	}

	return &leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: config.LeaseDuration.Duration,
		RenewDeadline: config.RenewDeadline.Duration,
		RetryPeriod:   config.RetryPeriod.Duration,
		WatchDog:      leaderelection.NewLeaderHealthzAdaptor(time.Second * 20),
		Name:          "kube-scheduler",
	}, nil
}

// createClients creates a kube client and an event client from the given config and masterOverride.
// TODO remove masterOverride when CLI flags are removed.
func createClients(config componentbaseconfig.ClientConnectionConfiguration, masterOverride string, timeout time.Duration) (clientset.Interface, clientset.Interface, clientset.Interface, error) {
	if len(config.Kubeconfig) == 0 && len(masterOverride) == 0 {
		klog.Warningf("Neither --kubeconfig nor --master was specified. Using default API client. This might not work.")
	}

	// This creates a client, first loading any specified kubeconfig
	// file, and then overriding the Master flag, if non-empty.
	kubeConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: config.Kubeconfig},
		&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: masterOverride}}).ClientConfig()
	if err != nil {
		return nil, nil, nil, err
	}

	kubeConfig.DisableCompression = true
	kubeConfig.AcceptContentTypes = config.AcceptContentTypes
	kubeConfig.ContentType = config.ContentType
	kubeConfig.QPS = config.QPS
	kubeConfig.Burst = int(config.Burst)

	client, err := clientset.NewForConfig(restclient.AddUserAgent(kubeConfig, "scheduler"))
	if err != nil {
		return nil, nil, nil, err
	}

	// shallow copy, do not modify the kubeConfig.Timeout.
	restConfig := *kubeConfig
	restConfig.Timeout = timeout
	leaderElectionClient, err := clientset.NewForConfig(restclient.AddUserAgent(&restConfig, "leader-election"))
	if err != nil {
		return nil, nil, nil, err
	}

	eventClient, err := clientset.NewForConfig(kubeConfig)
	if err != nil {
		return nil, nil, nil, err
	}

	return client, leaderElectionClient, eventClient, nil
}
