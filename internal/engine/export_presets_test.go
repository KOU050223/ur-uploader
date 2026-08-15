package engine

import (
	"os"
	"path/filepath"
	"testing"
)

// sampleCfg は Godot エディタが実際に書き出す形に近い cfg。
const sampleCfg = `[preset.0]

name="Windows Desktop"
platform="Windows Desktop"
runnable=true
export_filter="all_resources"
export_path="build/win/game.exe"

[preset.0.options]

binary_format/embed_pck=false
custom_template/debug=""

[preset.1]

name="Web"
platform="Web"
runnable=true
export_filter="all_resources"
export_path="build/web/mygame.html"

[preset.1.options]

html/export_icon=true
`

func writeCfg(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "export_presets.cfg")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadExportPresets(t *testing.T) {
	dir := writeCfg(t, sampleCfg)

	presets, err := LoadExportPresets(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(presets) != 2 {
		t.Fatalf("プリセット数が想定と異なる: %d", len(presets))
	}

	if presets[0].Name != "Windows Desktop" || presets[0].IsWeb() {
		t.Errorf("preset.0 の解析結果が不正: %+v", presets[0])
	}
	// [preset.N.options] の中身を拾ってしまっていないことを確認する。
	if presets[1].Name != "Web" || presets[1].Platform != "Web" {
		t.Errorf("preset.1 の解析結果が不正: %+v", presets[1])
	}
	if presets[1].ExportPath != "build/web/mygame.html" {
		t.Errorf("export_path が不正: %q", presets[1].ExportPath)
	}
	if !presets[1].IsWeb() {
		t.Error("Web プリセットと判定されない")
	}
}

func TestLoadExportPresetsMissing(t *testing.T) {
	if _, err := LoadExportPresets(t.TempDir()); err == nil {
		t.Fatal("cfg が無いのにエラーにならない")
	}
}

func TestPckPath(t *testing.T) {
	p := ExportPreset{ExportPath: "build/web/mygame.html"}
	got := p.PckPath("/proj")
	want := filepath.Join("/proj", "build", "web", "mygame.pck")
	if got != want {
		t.Errorf("PckPath = %q, want %q", got, want)
	}
}

func TestPckPathAbsolute(t *testing.T) {
	p := ExportPreset{ExportPath: "/out/game.html"}
	if got := p.PckPath("/proj"); got != filepath.Join("/out", "game.pck") {
		t.Errorf("絶対パスが尊重されない: %q", got)
	}
}

func TestSelectPreset(t *testing.T) {
	presets := []ExportPreset{
		{Name: "Windows Desktop", Platform: "Windows Desktop"},
		{Name: "Web", Platform: "Web"},
	}

	// 自動選択は Web 向けが1つだけなので成立する。
	p, err := SelectPreset(presets, "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Web" {
		t.Errorf("自動選択が不正: %q", p.Name)
	}

	// 明示指定は Web 以外でも通す。
	if p, err := SelectPreset(presets, "Windows Desktop"); err != nil || p.Name != "Windows Desktop" {
		t.Errorf("明示指定が通らない: %+v %v", p, err)
	}

	if _, err := SelectPreset(presets, "Nope"); err == nil {
		t.Error("存在しないプリセットがエラーにならない")
	}
}

func TestSelectPresetAmbiguous(t *testing.T) {
	presets := []ExportPreset{
		{Name: "Web", Platform: "Web"},
		{Name: "Web (itch)", Platform: "Web"},
	}
	if _, err := SelectPreset(presets, ""); err == nil {
		t.Error("Web が複数あるのにエラーにならない")
	}
}

func TestSelectPresetNoWeb(t *testing.T) {
	presets := []ExportPreset{{Name: "Windows Desktop", Platform: "Windows Desktop"}}
	if _, err := SelectPreset(presets, ""); err == nil {
		t.Error("Web が無いのにエラーにならない")
	}
}

// Godot 3 の HTML5 も Web 向けとして扱う。
func TestSelectPresetHTML5(t *testing.T) {
	presets := []ExportPreset{{Name: "HTML5", Platform: "HTML5"}}
	p, err := SelectPreset(presets, "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "HTML5" {
		t.Errorf("HTML5 が選ばれない: %q", p.Name)
	}
}

func TestResolveGodotPck(t *testing.T) {
	dir := writeCfg(t, sampleCfg)

	// --output 未指定なら cfg の export_path をそのまま使う。
	p, pck, err := ResolveGodotPck(dir, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Web" {
		t.Errorf("プリセットが不正: %q", p.Name)
	}
	if want := filepath.Join(dir, "build", "web", "mygame.pck"); pck != want {
		t.Errorf("pck = %q, want %q", pck, want)
	}

	// --output はディレクトリだけを差し替え、ファイル名は cfg に従う。
	_, pck, err = ResolveGodotPck(dir, "", "out")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "out", "mygame.pck"); pck != want {
		t.Errorf("--output 指定時の pck = %q, want %q", pck, want)
	}
}

func TestResolveGodotPckNoExportPath(t *testing.T) {
	dir := writeCfg(t, "[preset.0]\nname=\"Web\"\nplatform=\"Web\"\nexport_path=\"\"\n")
	if _, _, err := ResolveGodotPck(dir, "", ""); err == nil {
		t.Error("export_path が空なのにエラーにならない")
	}
}

func TestOutFileFor(t *testing.T) {
	got := outFileFor(filepath.Join("/o", "mygame.pck"), "build/web/mygame.html")
	if want := filepath.Join("/o", "mygame.html"); got != want {
		t.Errorf("outFileFor = %q, want %q", got, want)
	}
}
