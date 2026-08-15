package unityroom

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KOU050223/ur-uploader/internal/auth"
)

// uploaderProps はアップローダの data-props（HTMLエスケープ済み）を組み立てる。
func uploaderProps(uploadURL string) string {
	return `data-props="{&quot;targets&quot;:[{&quot;key&quot;:&quot;pck&quot;,` +
		`&quot;fileExtension&quot;:&quot;.pck&quot;,` +
		`&quot;uploadUrl&quot;:&quot;` + uploadURL + `&quot;,` +
		`&quot;contentType&quot;:&quot;application/octet-stream&quot;}],` +
		`&quot;uploadPath&quot;:&quot;/games/x/settings/webgl_upload&quot;,` +
		`&quot;csrfToken&quot;:&quot;TOKEN123&quot;}"`
}

// 別コンポーネントの data-props。アップローダより先に現れる想定。
const otherProps = `data-props="{&quot;items&quot;:[],&quot;label&quot;:&quot;nav&quot;}"`

func TestFetchProps_他のdata_propsを読み飛ばす(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// アップローダより前に無関係な data-props を置く。
		w.Write([]byte(`<div ` + otherProps + `></div><div ` + uploaderProps("https://example.com/up") + `></div>`))
	}))
	defer srv.Close()

	c := New(&auth.Credentials{Cookies: []auth.Cookie{{Name: "remember_token", Value: "x"}}})
	props, _, err := c.fetchPropsFrom(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("エラー: %v", err)
	}
	if props.CsrfToken != "TOKEN123" {
		t.Errorf("csrfToken = %q, want TOKEN123", props.CsrfToken)
	}
	if len(props.Targets) != 1 || props.Targets[0].Key != "pck" {
		t.Errorf("targets = %+v", props.Targets)
	}
}

func TestFetchProps_対象が無ければ設定を促す(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<div data-props="{&quot;targets&quot;:[],&quot;csrfToken&quot;:&quot;T&quot;,` +
			`&quot;uploadPath&quot;:&quot;/p&quot;}"></div>`))
	}))
	defer srv.Close()

	c := New(&auth.Credentials{Cookies: []auth.Cookie{{Name: "remember_token", Value: "x"}}})
	_, _, err := c.fetchPropsFrom(context.Background(), srv.URL)
	if err == nil || !strings.Contains(err.Error(), "エンジン") {
		t.Errorf("エンジン設定を促すエラーを期待したが: %v", err)
	}
}

func TestFetchProps_エンジン未選択を仕様変更と誤認しない(t *testing.T) {
	// エンジン未選択のとき、アップローダは描画されず data-props が1つも無い。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<body class="pg_games-settings-webgl_uploads pg_games-settings-webgl_uploads_show">` +
			`<div class="el_alert el_alert__warning">先に Webビルド設定 にて選択してください。</div></body>`))
	}))
	defer srv.Close()

	c := New(&auth.Credentials{Cookies: []auth.Cookie{{Name: "remember_token", Value: "x"}}})
	_, _, err := c.fetchPropsFrom(context.Background(), srv.URL)
	if err != errEngineNotSelected {
		t.Errorf("errEngineNotSelected を期待したが: %v", err)
	}
}

func TestFetchProps_アップロード画面でなければ仕様変更を疑う(t *testing.T) {
	// 目印が無い＝そもそも想定した画面ではないので、従来通りの案内にする。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<body class="pg_top"><div>まったく別の画面</div></body>`))
	}))
	defer srv.Close()

	c := New(&auth.Credentials{Cookies: []auth.Cookie{{Name: "remember_token", Value: "x"}}})
	_, _, err := c.fetchPropsFrom(context.Background(), srv.URL)
	if err == nil || !strings.Contains(err.Error(), "仕様変更") {
		t.Errorf("仕様変更を疑うエラーを期待したが: %v", err)
	}
}

func TestFetchProps_未ログインを検出する(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := New(&auth.Credentials{Cookies: []auth.Cookie{{Name: "remember_token", Value: "x"}}})
	_, _, err := c.fetchPropsFrom(context.Background(), srv.URL)
	if err != auth.ErrNotLoggedIn {
		t.Errorf("ErrNotLoggedIn を期待したが: %v", err)
	}
}

func TestUpload_ファイル不足なら送信前に中断する(t *testing.T) {
	var putCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			putCalled = true
		}
		w.Write([]byte(`<div ` + uploaderProps("http://"+r.Host+"/up") + `></div>`))
	}))
	defer srv.Close()

	c := New(&auth.Credentials{Cookies: []auth.Cookie{{Name: "remember_token", Value: "x"}}})
	// pck を渡さない。
	err := c.uploadTo(context.Background(), srv.URL, map[string]string{}, nil)
	if err == nil || !strings.Contains(err.Error(), "アップロードするファイルがありません") {
		t.Errorf("不足エラーを期待したが: %v", err)
	}
	if putCalled {
		t.Error("ファイル不足なのに PUT が実行された")
	}
}

func TestUpload_3ステップが正しい順で実行される(t *testing.T) {
	var steps []string
	var gotToken string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			steps = append(steps, "GET")
			http.SetCookie(w, &http.Cookie{Name: "_unity-room_session", Value: "updated"})
			w.Write([]byte(`<div ` + uploaderProps("http://"+r.Host+"/up") + `></div>`))
		case r.Method == http.MethodPut:
			steps = append(steps, "PUT")
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPatch:
			steps = append(steps, "PATCH")
			gotToken = r.Header.Get("X-CSRF-Token")
			// セッションが引き継がれているかを確認する。
			if ck, err := r.Cookie("_unity-room_session"); err != nil || ck.Value != "updated" {
				t.Errorf("更新後のセッションが引き継がれていない: %v", err)
			}
			w.Header().Set("Location", "/done")
			w.WriteHeader(http.StatusFound) // 成功は 302
		}
	}))
	defer srv.Close()

	pck := filepath.Join(t.TempDir(), "Build.pck")
	if err := os.WriteFile(pck, []byte("dummy"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := New(&auth.Credentials{Cookies: []auth.Cookie{{Name: "remember_token", Value: "x"}}})
	if err := c.uploadTo(context.Background(), srv.URL, map[string]string{"pck": pck}, nil); err != nil {
		t.Fatalf("エラー: %v", err)
	}

	if got := strings.Join(steps, ","); got != "GET,PUT,PATCH" {
		t.Errorf("順序 = %q, want GET,PUT,PATCH", got)
	}
	if gotToken != "TOKEN123" {
		t.Errorf("CSRFトークン = %q, want TOKEN123", gotToken)
	}
}
