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

// Package install installs the apps API group, making it available as
// an option to all of the API encoding/decoding machinery.
package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/apis/apps"
	"k8s.io/kubernetes/pkg/apis/apps/v1"
	"k8s.io/kubernetes/pkg/apis/apps/v1beta1"
	"k8s.io/kubernetes/pkg/apis/apps/v1beta2"
)

// install.go: 把当前资源组下的所有资源注册到资源注册表中.

func init() {
	// 把当前 apps 资源组下的所有资源注册到资源注册表中.
	Install(legacyscheme.Scheme)
}

// Install registers the API group and adds types to a scheme
func Install(scheme *runtime.Scheme) {
	// 注册 apps 资源组内部版本的资源
	utilruntime.Must(apps.AddToScheme(scheme))
	// 注册 v1beta1 外部版本下该所有资源的默认值函数和默认的内外版本转换函数
	utilruntime.Must(v1beta1.AddToScheme(scheme))
	// 注册 v1beta2 外部版本下该所有资源的默认值函数和默认的内外版本转换函数
	utilruntime.Must(v1beta2.AddToScheme(scheme))
	// 注册 v1 外部版本下该所有资源的默认值函数
	utilruntime.Must(v1.AddToScheme(scheme))
	// 注册资源组的版本顺序, 若有多个资源版本, 排在最前面的为资源首选版本.
	utilruntime.Must(scheme.SetVersionPriority(v1.SchemeGroupVersion, v1beta2.SchemeGroupVersion, v1beta1.SchemeGroupVersion))
}
