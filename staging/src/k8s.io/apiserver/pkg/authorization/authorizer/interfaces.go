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

package authorizer

import (
	"context"
	"net/http"

	"k8s.io/apiserver/pkg/authentication/user"
)

// Attributes is an interface used by an Authorizer to get information about a request
// that is used to make an authorization decision.
type Attributes interface {
	// GetUser returns the user.Info object to authorize
	GetUser() user.Info

	// GetVerb returns the kube verb associated with API requests (this includes get, list, watch, create, update, patch, delete, deletecollection, and proxy),
	// or the lowercased HTTP verb associated with non-API requests (this includes get, put, post, patch, and delete)
	GetVerb() string

	// When IsReadOnly() == true, the request has no side effects, other than
	// caching, logging, and other incidentals.
	IsReadOnly() bool

	// The namespace of the object, if a request is for a REST object.
	GetNamespace() string

	// The kind of object, if a request is for a REST object.
	GetResource() string

	// GetSubresource returns the subresource being requested, if present
	GetSubresource() string

	// GetName returns the name of the object as parsed off the request.  This will not be present for all request types, but
	// will be present for: get, update, delete
	GetName() string

	// The group of the resource, if a request is for a REST object.
	GetAPIGroup() string

	// GetAPIVersion returns the version of the group requested, if a request is for a REST object.
	GetAPIVersion() string

	// IsResourceRequest returns true for requests to API resources, like /api/v1/nodes,
	// and false for non-resource endpoints like /api, /healthz
	IsResourceRequest() bool

	// GetPath returns the path of the request
	GetPath() string
}

// Authorizer makes an authorization decision based on information gained by making
// zero or more calls to methods of the Attributes interface.  It returns nil when an action is
// authorized, otherwise it returns an error.
//
// Authorizer 授权器接口定义, 每一种授权机制都需要实现 Authorize 授权器接口方法
type Authorizer interface {
	// Attributes 参数是决定授权器从 HTTP 请求中获取授权信息方法的参数, 如 GetUser、GetVerb 等.
	Authorize(ctx context.Context, a Attributes) (authorized Decision, reason string, err error)
}

type AuthorizerFunc func(a Attributes) (Decision, string, error)

func (f AuthorizerFunc) Authorize(ctx context.Context, a Attributes) (Decision, string, error) {
	return f(a)
}

// RuleResolver provides a mechanism for resolving the list of rules that apply to a given user within a namespace.
// RuleResolver 授权器通过 RuleResolver 规则解析器去解析规则.
type RuleResolver interface {
	// RulesFor get the list of cluster wide rules, the list of rules in the specific namespace, incomplete status and errors.
	// 该方法通过接收的 user 用户信息及 namespace 命名空间参数, 解析出规则列表并返回.
	// 规则列表分两种:
	// - ResourceRuleInfo: 资源类型的规则列表, 如 /api/v1/pods 的资源接口
	// - NonResourceRuleInfo: 非资源类型的规则列表, 如 /api 或 /health 的资源接口
	RulesFor(user user.Info, namespace string) ([]ResourceRuleInfo, []NonResourceRuleInfo, bool, error)
}

// RequestAttributesGetter provides a function that extracts Attributes from an http.Request
type RequestAttributesGetter interface {
	GetRequestAttributes(user.Info, *http.Request) Attributes
}

// AttributesRecord implements Attributes interface.
type AttributesRecord struct {
	User            user.Info
	Verb            string
	Namespace       string
	APIGroup        string
	APIVersion      string
	Resource        string
	Subresource     string
	Name            string
	ResourceRequest bool
	Path            string
}

func (a AttributesRecord) GetUser() user.Info {
	return a.User
}

func (a AttributesRecord) GetVerb() string {
	return a.Verb
}

func (a AttributesRecord) IsReadOnly() bool {
	return a.Verb == "get" || a.Verb == "list" || a.Verb == "watch"
}

func (a AttributesRecord) GetNamespace() string {
	return a.Namespace
}

func (a AttributesRecord) GetResource() string {
	return a.Resource
}

func (a AttributesRecord) GetSubresource() string {
	return a.Subresource
}

func (a AttributesRecord) GetName() string {
	return a.Name
}

func (a AttributesRecord) GetAPIGroup() string {
	return a.APIGroup
}

func (a AttributesRecord) GetAPIVersion() string {
	return a.APIVersion
}

func (a AttributesRecord) IsResourceRequest() bool {
	return a.ResourceRequest
}

func (a AttributesRecord) GetPath() string {
	return a.Path
}

// Decision 决策状态, 类似于认证中的 true 和 false, 用于决定是否授权成功.
// 授权支持下面 3 种 Decision 决策状态, 如授权成功, 则返回 DecisionAllow 决策状态.
type Decision int

const (
	// DecisionDeny means that an authorizer decided to deny the action.
	// DecisionDeny 表示授权器拒绝该操作.
	DecisionDeny Decision = iota
	// DecisionAllow means that an authorizer decided to allow the action.
	// DecisionAllow 表示授权器允许该操作.
	DecisionAllow
	// DecisionNoOpionion means that an authorizer has no opinion on whether
	// to allow or deny an action.
	// DecisionNoOpinion 表示授权器对是否允许或拒绝某个操作没有意见, 会继续执行下一个授权器.
	DecisionNoOpinion
)
