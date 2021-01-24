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

package anonymous

import (
	"net/http"

	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/user"
)

const (
	anonymousUser = user.Anonymous

	unauthenticatedGroup = user.AllUnauthenticated
)

// Anonymous 认证就是匿名认证, 未被其他认证器拒绝的请求都可视为匿名请求. kube-apiserver
// 默认开启 Anonymous（匿名）认证.
//
// kube-apiserver 通过指定 --anonymous-auth 参数启用 Anonymous 认证, 默认该参数为 true

// Anonymous 认证的实现
func NewAuthenticator() authenticator.Request {
	return authenticator.RequestFunc(func(req *http.Request) (*authenticator.Response, bool, error) {
		// 在进行 Anonymous 认证时直接认证成功
		auds, _ := authenticator.AudiencesFrom(req.Context())
		return &authenticator.Response{
			User: &user.DefaultInfo{
				Name:   anonymousUser,
				Groups: []string{unauthenticatedGroup},
			},
			Audiences: auds,
		}, true, nil
	})
}
