package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetect(t *testing.T) {
	t.Run("godotを判定する", func(t *testing.T) {
		dir := t.TempDir()
		mustWrite(t, filepath.Join(dir, "project.godot"), "")

		eng, err := Detect(dir)
		if err != nil {
			t.Fatalf("Detect: %v", err)
		}
		if eng.Name() != "godot" {
			t.Errorf("Name = %q, want godot", eng.Name())
		}
	})

	t.Run("unityを判定する", func(t *testing.T) {
		dir := t.TempDir()
		mustMkdir(t, filepath.Join(dir, "Assets"))
		mustMkdir(t, filepath.Join(dir, "ProjectSettings"))

		eng, err := Detect(dir)
		if err != nil {
			t.Fatalf("Detect: %v", err)
		}
		if eng.Name() != "unity" {
			t.Errorf("Name = %q, want unity", eng.Name())
		}
	})

	t.Run("該当なしはエラー", func(t *testing.T) {
		if _, err := Detect(t.TempDir()); err == nil {
			t.Error("エラーを期待したが nil")
		}
	})

	t.Run("Assetsだけではunityと判定しない", func(t *testing.T) {
		dir := t.TempDir()
		mustMkdir(t, filepath.Join(dir, "Assets"))

		if _, err := Detect(dir); err == nil {
			t.Error("エラーを期待したが nil")
		}
	})
}

func TestByName(t *testing.T) {
	for _, name := range []string{"godot", "unity"} {
		eng, err := ByName(name)
		if err != nil {
			t.Errorf("ByName(%q): %v", name, err)
			continue
		}
		if eng.Name() != name {
			t.Errorf("Name = %q, want %q", eng.Name(), name)
		}
	}

	if _, err := ByName("unreal"); err == nil {
		t.Error("未対応エンジンはエラーになるべき")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
