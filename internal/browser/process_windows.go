//go:build windows

package browser

import "os"

// processAlive はプロセスが生きているかを返す。
//
// Windows では Signal(0) によるプロセス存在確認が使えないため、
// ここでは判定せず常に true を返す。
// ブラウザが閉じられた場合は WaitForLogin 側が CDP の連続エラーで検知する。
func processAlive(*os.Process) bool {
	return true
}
