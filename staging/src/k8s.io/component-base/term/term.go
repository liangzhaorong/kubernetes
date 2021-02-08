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

package term

import (
	"fmt"
	"io"

	"github.com/moby/term"
)

// TerminalSize returns the current width and height of the user's terminal. If it isn't a terminal,
// nil is returned. On error, zero values are returned for width and height.
// Usually w must be the stdout of the process. Stderr won't work.
//
// TerminalSize 返回用户终端的当前宽度和高度. 如果不是终端, 则返回 nil, 发生错误时, 将返回宽度和高度的 0 值.
// 通常输入参数 w 必须是当前进程的标准输出. stderr 无法正常工作.
func TerminalSize(w io.Writer) (int, int, error) {
	outFd, isTerminal := term.GetFdInfo(w)
	if !isTerminal {
		return 0, 0, fmt.Errorf("given writer is no terminal")
	}
	winsize, err := term.GetWinsize(outFd)
	if err != nil {
		return 0, 0, err
	}
	return int(winsize.Width), int(winsize.Height), nil
}
