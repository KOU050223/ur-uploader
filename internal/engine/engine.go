// Package engine はゲームエンジンごとのビルド処理を抽象化する。
package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// Artifact はビルド成果物。
// キーは unityroom 側のアップロード対象キー（Godot なら "pck"）、
// 値はローカルのファイルパス。
type Artifact struct {
	Files map[string]string
}

// Engine はエンジンごとのビルド処理。
type Engine interface {
	// Name は表示用の名前を返す。
	Name() string
	// Detect は dir がこのエンジンのプロジェクトかを判定する。
	Detect(dir string) bool
	// Build は Web 向けビルドを行い、成果物を返す。
	Build(ctx context.Context, dir string, opts BuildOptions) (*Artifact, error)
}

// BuildOptions はビルド時の設定。
type BuildOptions struct {
	// ExecPath はエンジン実行ファイルのパス。空ならPATHから探す。
	ExecPath string
	// OutputDir はビルド出力先。空ならエンジンごとの既定値。
	OutputDir string
	// Preset は Godot のエクスポートプリセット名。
	Preset string
	// BuildMethod は Unity の -executeMethod に渡す値。
	BuildMethod string
	// Verbose が真ならエンジンの出力をそのまま流す。
	Verbose bool
}

// All は対応する全エンジン。判定はこの順で行う。
func All() []Engine {
	return []Engine{
		&Godot{},
		&Unity{},
	}
}

// Detect は dir からエンジンを自動判定する。
func Detect(dir string) (Engine, error) {
	var found []Engine
	for _, e := range All() {
		if e.Detect(dir) {
			found = append(found, e)
		}
	}

	switch len(found) {
	case 0:
		return nil, fmt.Errorf(
			"エンジンを判定できません (%s)。\n"+
				"Godot は project.godot、Unity は Assets/ と ProjectSettings/ が必要です。\n"+
				"--engine で明示的に指定することもできます", dir)
	case 1:
		return found[0], nil
	default:
		return nil, fmt.Errorf(
			"複数のエンジンが検出されました。--engine で指定してください")
	}
}

// ByName は名前からエンジンを取得する。
func ByName(name string) (Engine, error) {
	for _, e := range All() {
		if e.Name() == name {
			return e, nil
		}
	}
	return nil, fmt.Errorf("未対応のエンジンです: %s", name)
}

// exists はパスの存在を確認する。
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// isDir はディレクトリの存在を確認する。
func isDir(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

// abs は dir 基準の絶対パスを返す。
func abs(dir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(dir, path)
}
