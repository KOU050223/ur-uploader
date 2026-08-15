// Package auth は unityroom の認証情報（Cookie）の保存と読み込みを扱う。
package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Cookie は保存対象の Cookie 1件。
type Cookie struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Credentials は保存された認証情報一式。
type Credentials struct {
	Cookies []Cookie `json:"cookies"`
}

// RequiredCookie は認証の実体となる Cookie 名。
// _unity-room_session はアクセスごとに再発行される一時的なものなので保存しない。
const RequiredCookie = "remember_token"

// ErrNotLoggedIn は認証情報が無い、または不完全な場合に返る。
var ErrNotLoggedIn = errors.New("ログインしていません。`ur-uploader login` を実行してください")

// Dir は認証情報の保存先ディレクトリを返す。
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("ホームディレクトリを取得できません: %w", err)
	}
	return filepath.Join(home, ".ur-uploader"), nil
}

// Path は認証情報ファイルのパスを返す。
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "auth.json"), nil
}

// Save は認証情報を保存する。認証情報のため 0600 で書き込む。
func Save(c *Credentials) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("保存先を作成できません: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("認証情報を変換できません: %w", err)
	}

	path := filepath.Join(dir, "auth.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("認証情報を保存できません: %w", err)
	}
	// WriteFile は既存ファイルのモードを変えないため、明示的に絞る。
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("認証情報の権限を設定できません: %w", err)
	}
	return nil
}

// Load は保存済みの認証情報を読み込む。
func Load() (*Credentials, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotLoggedIn
	}
	if err != nil {
		return nil, fmt.Errorf("認証情報を読み込めません: %w", err)
	}

	var c Credentials
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("認証情報が壊れています (%s): %w", path, err)
	}
	if !c.has(RequiredCookie) {
		return nil, ErrNotLoggedIn
	}
	return &c, nil
}

func (c *Credentials) has(name string) bool {
	for _, ck := range c.Cookies {
		if ck.Name == name && ck.Value != "" {
			return true
		}
	}
	return false
}

// Header は Cookie ヘッダー用の文字列を組み立てる。
func (c *Credentials) Header() string {
	parts := make([]string, 0, len(c.Cookies))
	for _, ck := range c.Cookies {
		parts = append(parts, ck.Name+"="+ck.Value)
	}
	return strings.Join(parts, "; ")
}

// Jar は Cookie を保持し、レスポンスの Set-Cookie を取り込むための入れ物。
//
// unityroom の CSRF トークンはセッションと対で検証されるため、
// GET で更新された _unity-room_session を後続の PATCH に引き継ぐ必要がある。
// これを怠ると 422 になる。
type Jar struct {
	values map[string]string
}

// NewJar は保存済み認証情報から Jar を作る。
func NewJar(c *Credentials) *Jar {
	j := &Jar{values: make(map[string]string, len(c.Cookies)+1)}
	for _, ck := range c.Cookies {
		j.values[ck.Name] = ck.Value
	}
	return j
}

// Absorb はレスポンスの Set-Cookie を取り込む。
func (j *Jar) Absorb(resp *http.Response) {
	for _, ck := range resp.Cookies() {
		if ck.Value == "" {
			delete(j.values, ck.Name)
			continue
		}
		j.values[ck.Name] = ck.Value
	}
}

// Header は現在の Cookie ヘッダー文字列を返す。
func (j *Jar) Header() string {
	names := make([]string, 0, len(j.values))
	for name := range j.values {
		names = append(names, name)
	}
	sort.Strings(names) // 出力を安定させる

	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, name+"="+j.values[name])
	}
	return strings.Join(parts, "; ")
}
