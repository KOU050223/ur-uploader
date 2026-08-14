// Command ur-uploader は unityroom へのゲームアップロードを自動化する非公式CLI。
package main

import "github.com/KOU050223/ur-uploader/internal/cli"

// version はビルド時に -ldflags で埋め込む。
var version = "dev"

func main() {
	cli.SetVersion(version)
	cli.Execute()
}
