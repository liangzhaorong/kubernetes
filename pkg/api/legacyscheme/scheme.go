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

package legacyscheme

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	// Scheme is the default instance of runtime.Scheme to which types in the Kubernetes API are already registered.
	// NOTE: If you are copying this file to start a new api group, STOP! Copy the
	// extensions group instead. This Scheme is special and should appear ONLY in
	// the api group, unless you really know what you're doing.
	// TODO(lavalamp): make the above error impossible.
	//
	// Kubernetes 系统拥有众多资源, 每一种资源就是一个资源类型, 这些资源类型需要有统一的注册、存储、查询、管理等机制. 目前
	// Kubernetes 系统中所有的资源类型都注册到 Scheme 资源注册表, 其是一个内存型的资源注册表.
	//
	// Scheme 资源注册表支持两种资源类型(Type)的注册, 分别是 UnversionedType 和 KnownType 资源类型.
	//
	// UnversionedType: 无版本资源类型, 这是一个早期 Kubernetes 系统中的概念, 它主要应用于某些没有版本的资源类型, 该类型的
	// 资源对象并不需要进行转换. 在目前的 Kubernets 发行版本中, 无版本类型已被弱化, 几乎所有的资源对象都拥有版本, 但在 metav1
	// 元数据中还有部分类型, 它们既属于 meta.k8s.io/v1 又属于 UnversionedType 无版本资源类型, 如 metav1.Status、
	// metav1.APIVersions、metav1.APIGroupList、metav1.APIGroup、metav1.APIResourceList.
	//
	// KnownType: 拥有版本的资源类型.
	//
	// Scheme 资源注册表
	Scheme = runtime.NewScheme()

	// Codecs provides access to encoding and decoding for the scheme
	//
	// Codecs 编解码器
	Codecs = serializer.NewCodecFactory(Scheme)

	// ParameterCodec handles versioning of objects that are converted to query parameters.
	// ParameterCodec 参数编解码器
	ParameterCodec = runtime.NewParameterCodec(Scheme)
)
