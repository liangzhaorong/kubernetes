/*
Copyright 2019 The Kubernetes Authors.

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

package framework

import (
	"errors"
	"sync"
)

const (
	// NotFound is the not found error message.
	NotFound = "not found"
)

// StateData is a generic type for arbitrary data stored in CycleState.
// StateData 是存储在 CycleState 中的任意数据的通用类型.
type StateData interface {
	// Clone is an interface to make a copy of StateData. For performance reasons,
	// clone should make shallow copies for members (e.g., slices or maps) that are not
	// impacted by PreFilter's optional AddPod/RemovePod methods.
	Clone() StateData
}

// StateKey is the type of keys stored in CycleState.
// StateKey 是存储在 CycleState 中的键的类型.
type StateKey string

// CycleState provides a mechanism for plugins to store and retrieve arbitrary data.
// StateData stored by one plugin can be read, altered, or deleted by another plugin.
// CycleState does not provide any data protection, as all plugins are assumed to be
// trusted.
//
// CycleState 为插件提供了一种存储和检索任意数据的机制.
// 一个插件存储的 StateData 可以被另一个插件读取，更改或删除.
// CycleState 不提供任何数据保护, 因为假定所有的插件都是可信的.
type CycleState struct {
	mx      sync.RWMutex
	storage map[StateKey]StateData
	// if recordPluginMetrics is true, PluginExecutionDuration will be recorded for this cycle.
	// 如果 recordPluginMetrics 为 true, 则将为此 cycle 记录 PluginExecutionDuration.
	recordPluginMetrics bool
}

// NewCycleState initializes a new CycleState and returns its pointer.
// NewCycleState 初始化一个 CycleState 对象并返回指向它的指针
func NewCycleState() *CycleState {
	return &CycleState{
		storage: make(map[StateKey]StateData),
	}
}

// ShouldRecordPluginMetrics returns whether PluginExecutionDuration metrics should be recorded.
func (c *CycleState) ShouldRecordPluginMetrics() bool {
	if c == nil {
		return false
	}
	return c.recordPluginMetrics
}

// SetRecordPluginMetrics sets recordPluginMetrics to the given value.
func (c *CycleState) SetRecordPluginMetrics(flag bool) {
	if c == nil {
		return
	}
	c.recordPluginMetrics = flag
}

// Clone creates a copy of CycleState and returns its pointer. Clone returns
// nil if the context being cloned is nil.
func (c *CycleState) Clone() *CycleState {
	if c == nil {
		return nil
	}
	copy := NewCycleState()
	for k, v := range c.storage {
		copy.Write(k, v.Clone())
	}
	return copy
}

// Read retrieves data with the given "key" from CycleState. If the key is not
// present an error is returned.
// This function is not thread safe. In multi-threaded code, lock should be
// acquired first.
//
// Read 从 CycleState 中检索具有给定 "key" 的数据. 如果该 key 不存在, 则返回错误.
// 该函数不是线程安全的. 在多线程代码中, 应首先获取锁.
func (c *CycleState) Read(key StateKey) (StateData, error) {
	if v, ok := c.storage[key]; ok {
		return v, nil
	}
	return nil, errors.New(NotFound)
}

// Write stores the given "val" in CycleState with the given "key".
// This function is not thread safe. In multi-threaded code, lock should be
// acquired first.
func (c *CycleState) Write(key StateKey, val StateData) {
	c.storage[key] = val
}

// Delete deletes data with the given key from CycleState.
// This function is not thread safe. In multi-threaded code, lock should be
// acquired first.
func (c *CycleState) Delete(key StateKey) {
	delete(c.storage, key)
}

// Lock acquires CycleState lock.
func (c *CycleState) Lock() {
	c.mx.Lock()
}

// Unlock releases CycleState lock.
func (c *CycleState) Unlock() {
	c.mx.Unlock()
}

// RLock acquires CycleState read lock.
func (c *CycleState) RLock() {
	c.mx.RLock()
}

// RUnlock releases CycleState read lock.
func (c *CycleState) RUnlock() {
	c.mx.RUnlock()
}
