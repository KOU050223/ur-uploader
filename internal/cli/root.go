// Package cli は ur-uploader のコマンドを定義する。
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version はビルド時に -ldflags で埋め込む。
var version = "dev"

// SetVersion はビルド時のバージョンを設定する。
func SetVersion(v string) {
	if v != "" {
		version = v
	}
}

// NewRootCommand はルートコマンドを作る。
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "ur-uploader",
		Short: "unityroom へのゲームアップロードを自動化する非公式CLI",
		Long: `ur-uploader は unityroom へのゲームアップロードを自動化する非公式ツールです。

Godot でビルドし、unityroom にアップロードするまでを1コマンドで実行します。

  ur-uploader login              初回のログイン（ブラウザが開きます）
  ur-uploader deploy <game-id>   ビルドしてアップロード

これは非公式ツールであり、unityroom 運営とは関係ありません。`,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		newLoginCommand(),
		newDeployCommand(),
		newUploadCommand(),
		newBuildCommand(),
	)
	return root
}

// Execute はコマンドを実行する。
func Execute() {
	if err := NewRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "エラー:", err)
		os.Exit(1)
	}
}
