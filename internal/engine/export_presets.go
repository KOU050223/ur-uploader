package engine

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// ExportPreset は export_presets.cfg のプリセット1件。
type ExportPreset struct {
	// Name はプリセット名（--export-release に渡す値）。
	Name string
	// Platform は "Web"（Godot 4）または "HTML5"（Godot 3）など。
	Platform string
	// ExportPath は cfg に書かれた出力先。プロジェクト相対または絶対。
	ExportPath string
}

// IsWeb は Web 向けのプリセットかを返す。
func (p ExportPreset) IsWeb() bool {
	switch p.Platform {
	case "Web", "HTML5":
		return true
	}
	return false
}

// PckPath は export_path から生成される pck の絶対パスを返す。
// Godot は出力先の basename をそのまま pck の名前に使う
// （例: build/mygame.html -> build/mygame.pck）。
func (p ExportPreset) PckPath(dir string) string {
	full := abs(dir, filepath.FromSlash(p.ExportPath))
	base := filepath.Base(full)
	base = strings.TrimSuffix(base, filepath.Ext(base)) + ".pck"
	return filepath.Join(filepath.Dir(full), base)
}

// exportPresetsFile は export_presets.cfg のパスを返す。
func exportPresetsFile(dir string) string {
	return filepath.Join(dir, "export_presets.cfg")
}

var (
	presetSectionRe = regexp.MustCompile(`^\[preset\.(\d+)\]$`)
	presetKeyRe     = regexp.MustCompile(`^(\w+)\s*=\s*(.*)$`)
)

// LoadExportPresets は export_presets.cfg を読み、プリセットを cfg の順で返す。
//
// 独自の INI 方言（[preset.N] と [preset.N.options] が交互に現れる）なので、
// 依存を増やさず必要なキーだけを拾う。
func LoadExportPresets(dir string) ([]ExportPreset, error) {
	path := exportPresetsFile(dir)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(
				"export_presets.cfg が見つかりません (%s)。\n"+
					"Godot エディタで Web 向けのエクスポートプリセットを作成してください", path)
		}
		return nil, fmt.Errorf("export_presets.cfg を読み込めません: %w", err)
	}
	defer f.Close()

	var presets []ExportPreset
	var cur *ExportPreset

	// [preset.N.options] 以降のキーは別物なので、この間は値を拾わない。
	inOptions := false

	flush := func() {
		if cur != nil {
			presets = append(presets, *cur)
			cur = nil
		}
	}

	sc := bufio.NewScanner(f)
	// 巨大な include_filter などがあっても読めるようにバッファを広げる。
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}

		if strings.HasPrefix(line, "[") {
			if presetSectionRe.MatchString(line) {
				flush()
				cur = &ExportPreset{}
				inOptions = false
			} else {
				// [preset.N.options] やその他のセクション。
				inOptions = true
			}
			continue
		}

		if cur == nil || inOptions {
			continue
		}

		m := presetKeyRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		val := unquoteCfg(m[2])
		switch m[1] {
		case "name":
			cur.Name = val
		case "platform":
			cur.Platform = val
		case "export_path":
			cur.ExportPath = val
		}
	}
	flush()

	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("export_presets.cfg を読み込めません: %w", err)
	}
	return presets, nil
}

// unquoteCfg は cfg の値からクォートを外す。
func unquoteCfg(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) {
		if v, err := strconv.Unquote(s); err == nil {
			return v
		}
		return s[1 : len(s)-1]
	}
	return s
}

// SelectPreset は使用するプリセットを決める。
//
// name が指定されていればそれを検証して返す。空なら Web 向けのプリセットが
// ちょうど1つの場合のみ自動選択し、0個や複数なら候補を挙げて明示指定を促す。
func SelectPreset(presets []ExportPreset, name string) (ExportPreset, error) {
	if name != "" {
		for _, p := range presets {
			if p.Name == name {
				return p, nil
			}
		}
		return ExportPreset{}, fmt.Errorf(
			"エクスポートプリセット %q が export_presets.cfg にありません。\n利用可能: %s",
			name, presetNames(presets))
	}

	var web []ExportPreset
	for _, p := range presets {
		if p.IsWeb() {
			web = append(web, p)
		}
	}

	switch len(web) {
	case 1:
		return web[0], nil
	case 0:
		return ExportPreset{}, fmt.Errorf(
			"Web 向けのエクスポートプリセットが export_presets.cfg にありません。\n"+
				"Godot エディタで Web プリセットを作成してください。\n利用可能: %s",
			presetNames(presets))
	default:
		return ExportPreset{}, fmt.Errorf(
			"Web 向けのプリセットが複数あります。--preset で指定してください。\n候補: %s",
			presetNames(web))
	}
}

// outFileFor は --export-release に渡す出力先を組み立てる。
//
// Godot に渡すのは pck ではなくエクスポート先本体（Web なら .html）なので、
// pck のディレクトリと basename に cfg の拡張子を戻して返す。
func outFileFor(pck, exportPath string) string {
	ext := filepath.Ext(filepath.FromSlash(exportPath))
	if ext == "" {
		ext = ".html"
	}
	base := strings.TrimSuffix(filepath.Base(pck), ".pck") + ext
	return filepath.Join(filepath.Dir(pck), base)
}

// presetNames は表示用にプリセット名を並べる。
func presetNames(presets []ExportPreset) string {
	if len(presets) == 0 {
		return "(なし)"
	}
	names := make([]string, 0, len(presets))
	for _, p := range presets {
		names = append(names, fmt.Sprintf("%q (%s)", p.Name, p.Platform))
	}
	return strings.Join(names, ", ")
}

// ResolveGodotPck はビルド成果物の pck パスを決める。
//
// outputDir が空なら export_presets.cfg の export_path をそのまま尊重する。
// 指定されていればディレクトリだけを差し替え、ファイル名は cfg に従う。
func ResolveGodotPck(dir, presetName, outputDir string) (ExportPreset, string, error) {
	presets, err := LoadExportPresets(dir)
	if err != nil {
		return ExportPreset{}, "", err
	}
	p, err := SelectPreset(presets, presetName)
	if err != nil {
		return ExportPreset{}, "", err
	}
	if p.ExportPath == "" {
		return ExportPreset{}, "", fmt.Errorf(
			"プリセット %q にエクスポート先が設定されていません。\n"+
				"Godot エディタでエクスポートパスを設定するか --output で指定してください", p.Name)
	}

	pck := p.PckPath(dir)
	if outputDir != "" {
		pck = filepath.Join(abs(dir, outputDir), filepath.Base(pck))
	}
	return p, pck, nil
}
