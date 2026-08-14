//go:build windows

package browser

import "os"

// processAlive はプロセスが生きているかを返す。
//
// Windows では Signal(0) が使えないため、CDP 接続の生死で判断する。
// ここでは常に true を返し、待機の打ち切りは CDP のエラーに任せる。
func processAlive(p *os.Process) bool {
	return true
}
