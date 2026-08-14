//go:build !windows

package browser

import (
	"os"
	"syscall"
)

// processAlive はプロセスが生きているかを返す。
// シグナル 0 は実際には送られず、存在確認のみを行う。
func processAlive(p *os.Process) bool {
	return p.Signal(syscall.Signal(0)) == nil
}
