package engine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Godot は Godot Engine のビルドを扱う。
type Godot struct{}

func (g *Godot) Name() string { return "godot" }

// Detect は project.godot の有無で判定する。
func (g *Godot) Detect(dir string) bool {
	return exists(filepath.Join(dir, "project.godot"))
}

// godotCandidates は実行ファイルの候補。
var godotCandidates = []string{
	"godot",
	"godot4",
	"/Applications/Godot.app/Contents/MacOS/Godot",
}

// resolveExec は Godot の実行ファイルを解決する。
func (g *Godot) resolveExec(explicit string) (string, error) {
	if explicit != "" {
		if _, err := exec.LookPath(explicit); err != nil {
			return "", fmt.Errorf("指定された Godot が見つかりません (%s): %w", explicit, err)
		}
		return explicit, nil
	}
	for _, c := range godotCandidates {
		if path, err := exec.LookPath(c); err == nil {
			return path, nil
		}
		if exists(c) {
			return c, nil
		}
	}
	return "", fmt.Errorf(
		"Godot が見つかりません。PATH に godot を通すか --godot-path で指定してください")
}

// Build は headless で Web エクスポートを行う。
func (g *Godot) Build(ctx context.Context, dir string, opts BuildOptions) (*Artifact, error) {
	bin, err := g.resolveExec(opts.ExecPath)
	if err != nil {
		return nil, err
	}

	preset := opts.Preset
	if preset == "" {
		preset = "Web"
	}
	outDir := opts.OutputDir
	if outDir == "" {
		outDir = "dist"
	}
	outDir = abs(dir, outDir)

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("出力先を作成できません: %w", err)
	}

	// unityroom は Build.pck という名前を要求するため、
	// エクスポート先を Build.html にして Build.pck を生成させる。
	// （エクスポート先の basename がそのまま pck の名前になる）
	outFile := filepath.Join(outDir, "Build.html")

	args := []string{
		"--headless",
		"--path", dir,
		"--export-release", preset,
		outFile,
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	var out strings.Builder
	if opts.Verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = &out
		cmd.Stderr = &out
	}

	if err := cmd.Run(); err != nil {
		if !opts.Verbose {
			return nil, fmt.Errorf("Godot のビルドに失敗しました: %w\n%s", err, tail(out.String(), 20))
		}
		return nil, fmt.Errorf("Godot のビルドに失敗しました: %w", err)
	}

	pck := filepath.Join(outDir, "Build.pck")
	if !exists(pck) {
		// プリセット名やエクスポート設定が想定と違うと pck が出ないことがある。
		return nil, fmt.Errorf(
			"ビルド成果物が見つかりません (%s)。\n"+
				"エクスポートプリセット %q が Web 向けに設定されているか確認してください", pck, preset)
	}

	return &Artifact{Files: map[string]string{"pck": pck}}, nil
}

// tail は文字列の末尾 n 行を返す。
func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
