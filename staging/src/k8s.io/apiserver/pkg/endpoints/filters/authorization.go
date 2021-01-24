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

package filters

import (
	"context"
	"errors"
	"net/http"

	"k8s.io/klog/v2"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/audit"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	"k8s.io/apiserver/pkg/endpoints/request"
)

const (
	// Annotation key names set in advanced audit
	decisionAnnotationKey = "authorization.k8s.io/decision"
	reasonAnnotationKey   = "authorization.k8s.io/reason"

	// Annotation values set in advanced audit
	decisionAllow  = "allow"
	decisionForbid = "forbid"
	reasonError    = "internal error"
)

// WithAuthorizationCheck passes all authorized requests on to handler, and returns a forbidden error otherwise.
// WithAuthorization 是 kube-apiserver 的授权 Handler 方法.
func WithAuthorization(handler http.Handler, a authorizer.Authorizer, s runtime.NegotiatedSerializer) http.Handler {
	// 如果 a 授权器为 nil, 则说明 kube-apiserver 未启用任何授权功能
	if a == nil {
		klog.Warningf("Authorization is disabled")
		return handler
	}
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		ae := request.AuditEventFrom(ctx)

		// 从 HTTP 请求中获取客户端信息
		attributes, err := GetAuthorizerAttributes(ctx)
		if err != nil {
			responsewriters.InternalError(w, req, err)
			return
		}
		// 在 a.Authorize 对请求进行授权过程中, 遍历已启用的授权器列表并执行授权器.
		authorized, reason, err := a.Authorize(ctx, attributes)
		// an authorizer like RBAC could encounter evaluation errors and still allow the request, so authorizer decision is checked before error here.
		// 如果授权成功, 则进入准入控制器阶段
		if authorized == authorizer.DecisionAllow {
			audit.LogAnnotation(ae, decisionAnnotationKey, decisionAllow)
			audit.LogAnnotation(ae, reasonAnnotationKey, reason)
			handler.ServeHTTP(w, req)
			return
		}
		if err != nil {
			audit.LogAnnotation(ae, reasonAnnotationKey, reasonError)
			responsewriters.InternalError(w, req, err)
			return
		}

		// 若授权失败, 则通过 responsewriters.Forbidden 函数返回 HTTP 401 Unauthorized 并返回授权失败的原因.
		klog.V(4).Infof("Forbidden: %#v, Reason: %q", req.RequestURI, reason)
		audit.LogAnnotation(ae, decisionAnnotationKey, decisionForbid)
		audit.LogAnnotation(ae, reasonAnnotationKey, reason)
		responsewriters.Forbidden(ctx, attributes, w, req, reason, s)
	})
}

func GetAuthorizerAttributes(ctx context.Context) (authorizer.Attributes, error) {
	attribs := authorizer.AttributesRecord{}

	user, ok := request.UserFrom(ctx)
	if ok {
		attribs.User = user
	}

	requestInfo, found := request.RequestInfoFrom(ctx)
	if !found {
		return nil, errors.New("no RequestInfo found in the context")
	}

	// Start with common attributes that apply to resource and non-resource requests
	attribs.ResourceRequest = requestInfo.IsResourceRequest
	attribs.Path = requestInfo.Path
	attribs.Verb = requestInfo.Verb

	attribs.APIGroup = requestInfo.APIGroup
	attribs.APIVersion = requestInfo.APIVersion
	attribs.Resource = requestInfo.Resource
	attribs.Subresource = requestInfo.Subresource
	attribs.Namespace = requestInfo.Namespace
	attribs.Name = requestInfo.Name

	return &attribs, nil
}
