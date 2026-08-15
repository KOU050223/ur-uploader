package auth

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJar_SetCookieを取り込む(t *testing.T) {
	// CSRF検証のため、GETで更新されたセッションを引き継ぐ必要がある。
	jar := NewJar(&Credentials{Cookies: []Cookie{
		{Name: "remember_token", Value: "tok"},
	}})

	resp := &http.Response{Header: http.Header{}}
	resp.Header.Add("Set-Cookie", "_unity-room_session=updated; path=/")
	jar.Absorb(resp)

	got := jar.Header()
	if !strings.Contains(got, "remember_token=tok") {
		t.Errorf("元のCookieが消えている: %q", got)
	}
	if !strings.Contains(got, "_unity-room_session=updated") {
		t.Errorf("更新分が取り込まれていない: %q", got)
	}
}

func TestJar_空値のCookieは削除する(t *testing.T) {
	jar := NewJar(&Credentials{Cookies: []Cookie{
		{Name: "remember_token", Value: "tok"},
		{Name: "temp", Value: "x"},
	}})

	resp := &http.Response{Header: http.Header{}}
	resp.Header.Add("Set-Cookie", "temp=; path=/")
	jar.Absorb(resp)

	if strings.Contains(jar.Header(), "temp=") {
		t.Errorf("削除されていない: %q", jar.Header())
	}
}

func TestSaveLoad(t *testing.T) {
	// 保存先をテスト用に差し替える。
	t.Setenv("HOME", t.TempDir())

	want := &Credentials{Cookies: []Cookie{
		{Name: "remember_token", Value: "tok"},
		{Name: "user_id", Value: "42"},
	}}
	if err := Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Cookies) != 2 || got.Cookies[0].Value != "tok" {
		t.Errorf("読み込み結果が不正: %+v", got.Cookies)
	}
}

func TestSave_権限を0600にする(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// 先に緩い権限のファイルを作っておく。
	// os.WriteFile は既存ファイルのモードを変えないため、
	// Chmod していないとここで漏れる。
	dir := filepath.Join(home, ".ur-uploader")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "auth.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Save(&Credentials{Cookies: []Cookie{{Name: "remember_token", Value: "t"}}}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := st.Mode().Perm(); perm != 0o600 {
		t.Errorf("権限 = %o, want 600", perm)
	}
}

func TestLoad_未ログインを検出する(t *testing.T) {
	t.Run("ファイルが無い", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		if _, err := Load(); err != ErrNotLoggedIn {
			t.Errorf("ErrNotLoggedIn を期待したが: %v", err)
		}
	})

	t.Run("remember_tokenが無い", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		if err := Save(&Credentials{Cookies: []Cookie{{Name: "user_id", Value: "1"}}}); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(); err != ErrNotLoggedIn {
			t.Errorf("ErrNotLoggedIn を期待したが: %v", err)
		}
	})
}
