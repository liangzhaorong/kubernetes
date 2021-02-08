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

package options

import (
	"fmt"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/klog/v2"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/authorization/authorizerfactory"
	"k8s.io/apiserver/pkg/authorization/path"
	"k8s.io/apiserver/pkg/authorization/union"
	"k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// DelegatingAuthorizationOptions provides an easy way for composing API servers to delegate their authorization to
// the root kube API server.
// WARNING: never assume that every authenticated incoming request already does authorization.
//          The aggregator in the kube API server does this today, but this behaviour is not
//          guaranteed in the future.
//
// 授权相关参数
type DelegatingAuthorizationOptions struct {
	// RemoteKubeConfigFile is the file to use to connect to a "normal" kube API server which hosts the
	// SubjectAccessReview.authorization.k8s.io endpoint for checking tokens.
	//
	// --authorization-kubeconfig: 用于指定 kube-scheduler 的授权 kubeconfig 配置文件, 该文件中定义了访问
	// Kubernetes API Server 的配置信息.
	RemoteKubeConfigFile string
	// RemoteKubeConfigFileOptional is specifying whether not specifying the kubeconfig or
	// a missing in-cluster config will be fatal.
	RemoteKubeConfigFileOptional bool

	// AllowCacheTTL is the length of time that a successful authorization response will be cached
	//
	// --authorization-webhook-cache-authorized-ttl: 用于设置从 Webhook 授权服务中缓存已授权响应的缓存时间 (默认值为 10 秒)
	AllowCacheTTL time.Duration

	// DenyCacheTTL is the length of time that an unsuccessful authorization response will be cached.
	// You generally want more responsive, "deny, try again" flows.
	//
	// --authorization-webhook-cache-unauthorized-ttl: 用于设置从 Webhook 授权服务中缓存未授权响应的缓存时间 (默认值为 10秒)
	DenyCacheTTL time.Duration

	// AlwaysAllowPaths are HTTP paths which are excluded from authorization. They can be plain
	// paths or end in * in which case prefix-match is applied. A leading / is optional.
	//
	// AlwaysAllowPaths 指定从授权中排除的路径. 它们可能是纯路径 或者以 * 结尾, 在这种情况下, 将应用前缀匹配.
	// 此外, 前导 / 是可选的.
	//
	// --authorization-always-allow-paths: 用于设置不受授权权限影响的 HTTP 路径 (默认值为 /healthz)
	AlwaysAllowPaths []string

	// AlwaysAllowGroups are groups which are allowed to take any actions.  In kube, this is system:masters.
	AlwaysAllowGroups []string

	// ClientTimeout specifies a time limit for requests made by SubjectAccessReviews client.
	// The default value is set to 10 seconds.
	ClientTimeout time.Duration

	// WebhookRetryBackoff specifies the backoff parameters for the authorization webhook retry logic.
	// This allows us to configure the sleep time at each iteration and the maximum number of retries allowed
	// before we fail the webhook call in order to limit the fan out that ensues when the system is degraded.
	WebhookRetryBackoff *wait.Backoff
}

// 授权相关参数
func NewDelegatingAuthorizationOptions() *DelegatingAuthorizationOptions {
	return &DelegatingAuthorizationOptions{
		// very low for responsiveness, but high enough to handle storms
		AllowCacheTTL:       10 * time.Second,
		DenyCacheTTL:        10 * time.Second,
		ClientTimeout:       10 * time.Second,
		WebhookRetryBackoff: DefaultAuthWebhookRetryBackoff(),
	}
}

// WithAlwaysAllowGroups appends the list of paths to AlwaysAllowGroups
func (s *DelegatingAuthorizationOptions) WithAlwaysAllowGroups(groups ...string) *DelegatingAuthorizationOptions {
	s.AlwaysAllowGroups = append(s.AlwaysAllowGroups, groups...)
	return s
}

// WithAlwaysAllowPaths appends the list of paths to AlwaysAllowPaths
func (s *DelegatingAuthorizationOptions) WithAlwaysAllowPaths(paths ...string) *DelegatingAuthorizationOptions {
	s.AlwaysAllowPaths = append(s.AlwaysAllowPaths, paths...)
	return s
}

// WithClientTimeout sets the given timeout for SAR client used by this authorizer
func (s *DelegatingAuthorizationOptions) WithClientTimeout(timeout time.Duration) {
	s.ClientTimeout = timeout
}

// WithCustomRetryBackoff sets the custom backoff parameters for the authorization webhook retry logic.
func (s *DelegatingAuthorizationOptions) WithCustomRetryBackoff(backoff wait.Backoff) {
	s.WebhookRetryBackoff = &backoff
}

func (s *DelegatingAuthorizationOptions) Validate() []error {
	allErrors := []error{}

	if s.WebhookRetryBackoff != nil && s.WebhookRetryBackoff.Steps <= 0 {
		allErrors = append(allErrors, fmt.Errorf("number of webhook retry attempts must be greater than 1, but is: %d", s.WebhookRetryBackoff.Steps))
	}

	return allErrors
}

// AddFlags 添加授权相关参数
func (s *DelegatingAuthorizationOptions) AddFlags(fs *pflag.FlagSet) {
	if s == nil {
		return
	}

	var optionalKubeConfigSentence string
	if s.RemoteKubeConfigFileOptional {
		optionalKubeConfigSentence = " This is optional. If empty, all requests not skipped by authorization are forbidden."
	}
	// --authorization-kubeconfig: 用于指定 kube-scheduler 的授权 kubeconfig 配置文件, 该文件中定义了访问
	// Kubernetes API Server 的配置信息.
	fs.StringVar(&s.RemoteKubeConfigFile, "authorization-kubeconfig", s.RemoteKubeConfigFile,
		"kubeconfig file pointing at the 'core' kubernetes server with enough rights to create "+
			"subjectaccessreviews.authorization.k8s.io."+optionalKubeConfigSentence)

	// --authorization-webhook-cache-authorized-ttl: 用于设置从 Webhook 授权服务中缓存已授权响应的缓存时间 (默认值为 10 秒)
	fs.DurationVar(&s.AllowCacheTTL, "authorization-webhook-cache-authorized-ttl",
		s.AllowCacheTTL,
		"The duration to cache 'authorized' responses from the webhook authorizer.")

	// --authorization-webhook-cache-unauthorized-ttl: 用于设置从 Webhook 授权服务中缓存未授权响应的缓存时间 (默认值为 10秒)
	fs.DurationVar(&s.DenyCacheTTL,
		"authorization-webhook-cache-unauthorized-ttl", s.DenyCacheTTL,
		"The duration to cache 'unauthorized' responses from the webhook authorizer.")

	// --authorization-always-allow-paths: 用于设置不受授权权限影响的 HTTP 路径 (默认值为 /healthz)
	fs.StringSliceVar(&s.AlwaysAllowPaths, "authorization-always-allow-paths", s.AlwaysAllowPaths,
		"A list of HTTP paths to skip during authorization, i.e. these are authorized without "+
			"contacting the 'core' kubernetes server.")
}

func (s *DelegatingAuthorizationOptions) ApplyTo(c *server.AuthorizationInfo) error {
	if s == nil {
		c.Authorizer = authorizerfactory.NewAlwaysAllowAuthorizer()
		return nil
	}

	client, err := s.getClient()
	if err != nil {
		return err
	}

	c.Authorizer, err = s.toAuthorizer(client)
	return err
}

func (s *DelegatingAuthorizationOptions) toAuthorizer(client kubernetes.Interface) (authorizer.Authorizer, error) {
	var authorizers []authorizer.Authorizer

	if len(s.AlwaysAllowGroups) > 0 {
		authorizers = append(authorizers, authorizerfactory.NewPrivilegedGroups(s.AlwaysAllowGroups...))
	}

	if len(s.AlwaysAllowPaths) > 0 {
		a, err := path.NewAuthorizer(s.AlwaysAllowPaths)
		if err != nil {
			return nil, err
		}
		authorizers = append(authorizers, a)
	}

	if client == nil {
		klog.Warningf("No authorization-kubeconfig provided, so SubjectAccessReview of authorization tokens won't work.")
	} else {
		cfg := authorizerfactory.DelegatingAuthorizerConfig{
			SubjectAccessReviewClient: client.AuthorizationV1().SubjectAccessReviews(),
			AllowCacheTTL:             s.AllowCacheTTL,
			DenyCacheTTL:              s.DenyCacheTTL,
			WebhookRetryBackoff:       s.WebhookRetryBackoff,
		}
		delegatedAuthorizer, err := cfg.New()
		if err != nil {
			return nil, err
		}
		authorizers = append(authorizers, delegatedAuthorizer)
	}

	return union.New(authorizers...), nil
}

func (s *DelegatingAuthorizationOptions) getClient() (kubernetes.Interface, error) {
	var clientConfig *rest.Config
	var err error
	if len(s.RemoteKubeConfigFile) > 0 {
		loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: s.RemoteKubeConfigFile}
		loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})

		clientConfig, err = loader.ClientConfig()
	} else {
		// without the remote kubeconfig file, try to use the in-cluster config.  Most addon API servers will
		// use this path. If it is optional, ignore errors.
		clientConfig, err = rest.InClusterConfig()
		if err != nil && s.RemoteKubeConfigFileOptional {
			if err != rest.ErrNotInCluster {
				klog.Warningf("failed to read in-cluster kubeconfig for delegated authorization: %v", err)
			}
			return nil, nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get delegated authorization kubeconfig: %v", err)
	}

	// set high qps/burst limits since this will effectively limit API server responsiveness
	clientConfig.QPS = 200
	clientConfig.Burst = 400
	clientConfig.Timeout = s.ClientTimeout

	return kubernetes.NewForConfig(clientConfig)
}
