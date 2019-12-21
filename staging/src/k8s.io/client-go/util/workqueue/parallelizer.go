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

package workqueue

import (
	"context"
	"sync"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

type DoWorkPieceFunc func(piece int)

// ParallelizeUntil is a framework that allows for parallelizing N
// independent pieces of work until done or the context is canceled.
// ParallelizeUntil 是一个 framework (框架), 允许并行执行 N 个独立的 work pieces.
// 注: 默认传进来的 workers 值为 16，pieces 是当前集群的所有节点数
func ParallelizeUntil(ctx context.Context, workers, pieces int, doWorkPiece DoWorkPieceFunc) {
	var stop <-chan struct{}
	if ctx != nil {
		stop = ctx.Done()
	}

	// pieces 代表集群的节点数，因此 toProcess channel 中将缓存 pieces 个数据
	toProcess := make(chan int, pieces)
	for i := 0; i < pieces; i++ {
		toProcess <- i
	}
	// 关闭一个带有缓冲的 channel，这样会在 channel 中所有数据都被取出来后才会被关闭
	close(toProcess)

	// 若集群节点数小于 workers(默认 16)，则设置将要并行执行的 worker 数等于 pieces
	if pieces < workers {
		workers = pieces
	}

	wg := sync.WaitGroup{}
	wg.Add(workers)
	// 当集群节点数 pieces < 并发协程 goroutine 数 workers(默认 16) 时:
	//   那么下面将会启动的 goroutine 数等于 pieces 数, 即此时每个节点都会使用一个
	//   goroutine 进行 predicates 处理.
	// 当 pieces > workers(默认 16) 时:
	//   那么将会启动 16 个 goroutine, 即表示将有 16 个 goroutine 并发处理 pieces 个集群节点
	for i := 0; i < workers; i++ {
		go func() {
			defer utilruntime.HandleCrash()
			defer wg.Done()
			for piece := range toProcess {
				select {
				case <-stop:
					return
				default:
					doWorkPiece(piece)
				}
			}
		}()
	}
	wg.Wait()
}
