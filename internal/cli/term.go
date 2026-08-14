package cli

import (
	"fmt"
	"os"
)

// isTTY は標準出力が端末かどうか。
// パイプやリダイレクト時は進捗表示を出さない。
var isTTY = func() bool {
	st, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}()

// progressf は進捗を上書き表示する。端末でなければ何もしない。
func progressf(format string, a ...any) {
	if !isTTY {
		return
	}
	fmt.Printf("\r\033[K"+format, a...)
}

// clearLine は進捗表示を消す。
func clearLine() {
	if isTTY {
		fmt.Print("\r\033[K")
	}
}

// donef は進捗行を消してから結果を表示する。
func donef(format string, a ...any) {
	clearLine()
	fmt.Printf(format, a...)
}
