package engine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Unity は Unity のビルドを扱う。
//
// 注意: unityroom 側のアップロード対象（targets）の仕様が未確認のため、
// アップロードまでは未対応。ビルドまでは動作する。
type Unity struct{}

func (u *Unity) Name() string { return "unity" }

// Detect は Unity プロジェクトの構成で判定する。
func (u *Unity) Detect(dir string) bool {
	return isDir(filepath.Join(dir, "Assets")) &&
		isDir(filepath.Join(dir, "ProjectSettings"))
}

// resolveExec は Unity の実行ファイルを解決する。
//
// Unity はバージョンごとに別パスへ入るため、自動検出は Unity Hub の
// 既定の配置のみを見る。確実なのは --unity-path での明示指定。
func (u *Unity) resolveExec(explicit string) (string, error) {
	if explicit != "" {
		if !exists(explicit) {
			return "", fmt.Errorf("指定された Unity が見つかりません: %s", explicit)
		}
		return explicit, nil
	}

	if path, err := exec.LookPath("unity"); err == nil {
		return path, nil
	}

	// Unity Hub の既定の配置から探す（新しいバージョンを優先）。
	hub := "/Applications/Unity/Hub/Editor"
	entries, err := os.ReadDir(hub)
	if err == nil {
		var found []string
		for _, e := range entries {
			p := filepath.Join(hub, e.Name(), "Unity.app/Contents/MacOS/Unity")
			if exists(p) {
				found = append(found, p)
			}
		}
		if len(found) > 0 {
			return found[len(found)-1], nil
		}
	}

	return "", fmt.Errorf(
		"Unity が見つかりません。--unity-path で実行ファイルを指定してください")
}

// Build は batchmode で WebGL ビルドを行う。
func (u *Unity) Build(ctx context.Context, dir string, opts BuildOptions) (*Artifact, error) {
	bin, err := u.resolveExec(opts.ExecPath)
	if err != nil {
		return nil, err
	}

	method := opts.BuildMethod
	if method == "" {
		return nil, fmt.Errorf(
			"Unity のビルドには --build-method の指定が必要です。\n" +
				"例: --build-method BuildScript.BuildWebGL\n" +
				"プロジェクト側に WebGL ビルドを行う静的メソッドを用意してください")
	}

	outDir := opts.OutputDir
	if outDir == "" {
		outDir = filepath.Join("Build", "WebGL")
	}
	outDir = abs(dir, outDir)

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("出力先を作成できません: %w", err)
	}

	logFile := filepath.Join(outDir, "unity-build.log")
	args := []string{
		"-batchmode",
		"-quit",
		"-nographics",
		"-projectPath", dir,
		"-buildTarget", "WebGL",
		"-executeMethod", method,
		"-logFile", logFile,
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	if opts.Verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		hint := ""
		if data, rerr := os.ReadFile(logFile); rerr == nil {
			hint = "\n" + tail(string(data), 20)
		}
		return nil, fmt.Errorf("Unity のビルドに失敗しました: %w%s", err, hint)
	}

	// TODO: unityroom の Unity 向け targets の仕様が判明したら、
	// 生成物（.wasm/.data/.framework.js/.loader.js 等）を対応キーに割り当てる。
	// 現状は仕様が未確認のため、ここで明示的に打ち切る。
	return nil, fmt.Errorf(
		"ビルドは成功しました (%s)。\n"+
			"ただし unityroom への Unity ビルドのアップロードは未対応です。\n"+
			"現在は Godot のみ対応しています", outDir)
}

// unityArtifactHint は将来の実装用に、生成物の一覧を返す。
func unityArtifactHint(outDir string) []string {
	var found []string
	filepath.Walk(outDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		switch {
		case strings.HasSuffix(path, ".wasm"), strings.HasSuffix(path, ".data"),
			strings.HasSuffix(path, ".js"), strings.HasSuffix(path, ".json"):
			found = append(found, path)
		}
		return nil
	})
	return found
}
