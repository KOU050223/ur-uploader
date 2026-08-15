// Package unityroom は unityroom への Web ビルドアップロードを行う。
//
// 公式APIではなく、Web UI が内部で使っているエンドポイントを利用する非公式実装。
// アップロードは以下の3ステップで行われる:
//
//	① GET   アップロード画面から署名付きURLとCSRFトークンを取得
//	② PUT   オブジェクトストレージへ直接アップロード（unityroomを経由しない）
//	③ PATCH unityroom へ完了を通知
package unityroom

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/KOU050223/ur-uploader/internal/auth"
)

// BaseURL は unityroom のベースURL。
const BaseURL = "https://unityroom.com"

// Client は unityroom への操作を行う。
type Client struct {
	http  *http.Client
	creds *auth.Credentials
}

// New は Client を作る。
func New(creds *auth.Credentials) *Client {
	return &Client{
		// アップロードは時間がかかりうるのでタイムアウトは長めに取る。
		http: &http.Client{
			Timeout: 30 * time.Minute,
			// PATCH の成功応答は 302 であり、追ってはいけない。
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		creds: creds,
	}
}

// Target はアップロード対象のファイル1件。
type Target struct {
	Key             string `json:"key"`
	FileExtension   string `json:"fileExtension"`
	UploadURL       string `json:"uploadUrl"`
	ContentType     string `json:"contentType"`
	ContentEncoding string `json:"contentEncoding"`
}

// uploadProps はアップロード画面の data-props の内容。
type uploadProps struct {
	Targets    []Target `json:"targets"`
	UploadPath string   `json:"uploadPath"`
	CsrfToken  string   `json:"csrfToken"`
}

var dataPropsRe = regexp.MustCompile(`data-props="([^"]*)"`)

// uploadPageURL はアップロード画面のURLを組み立てる。
func uploadPageURL(permalink string) string {
	return fmt.Sprintf("%s/games/%s/settings/webgl_upload", BaseURL, permalink)
}

// fetchProps は①を行い、署名付きURLとCSRFトークンを取得する。
// 併せて、後続で使う Jar（更新されたセッションを含む）を返す。
// errGameNotFound は 404 を表す内部エラー。
var errGameNotFound = fmt.Errorf("game not found")

// fetchPropsFrom は指定URLから props を取得する（テストのために分離）。
func (c *Client) fetchPropsFrom(ctx context.Context, pageURL string) (*uploadProps, *auth.Jar, error) {
	jar := auth.NewJar(c.creds)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Cookie", jar.Header())

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("アップロード画面を取得できません: %w", err)
	}
	defer resp.Body.Close()

	// セッションの更新を必ず取り込む（CSRF検証に必要）。
	jar.Absorb(resp)

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusFound:
		return nil, nil, auth.ErrNotLoggedIn
	case http.StatusNotFound:
		return nil, nil, errGameNotFound
	default:
		return nil, nil, fmt.Errorf("アップロード画面の取得に失敗しました (status=%d)", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("応答を読み取れません: %w", err)
	}

	// data-props は他の React コンポーネントにも付いているため、
	// アップローダのものを内容で見分ける必要がある。
	matches := dataPropsRe.FindAllSubmatch(body, -1)
	if len(matches) == 0 {
		return nil, nil, fmt.Errorf(
			"アップロード情報が見つかりません。unityroom側の仕様変更の可能性があります")
	}

	found := false // アップローダらしき data-props が1つでもあったか
	for _, m := range matches {
		var props uploadProps
		if err := json.Unmarshal([]byte(html.UnescapeString(string(m[1]))), &props); err != nil {
			continue // 別コンポーネントの props なので読み飛ばす
		}
		if props.CsrfToken == "" && props.UploadPath == "" {
			continue
		}
		found = true
		if len(props.Targets) > 0 && props.CsrfToken != "" {
			return &props, jar, nil
		}
	}

	if found {
		// アップロード画面ではあるが、対象が無い状態。
		return nil, nil, fmt.Errorf("アップロード対象がありません。" +
			"unityroom側でエンジン（Godot/Unity）が設定されているか確認してください")
	}
	return nil, nil, fmt.Errorf(
		"アップロード情報が見つかりません。unityroom側の仕様変更の可能性があります")
}

// ProgressFunc はアップロード進捗の通知を受ける。
type ProgressFunc func(key string, sent, total int64)

// Upload はビルド成果物を unityroom にアップロードする。
// files は Target.Key に対応するローカルファイルのパス。
func (c *Client) Upload(ctx context.Context, permalink string, files map[string]string, onProgress ProgressFunc) error {
	if err := c.uploadTo(ctx, uploadPageURL(permalink), files, onProgress); err != nil {
		if err == errGameNotFound {
			return fmt.Errorf("ゲームが見つかりません: %s", permalink)
		}
		return err
	}
	return nil
}

// uploadTo は指定URLに対してアップロードを行う（テストのために分離）。
func (c *Client) uploadTo(ctx context.Context, pageURL string, files map[string]string, onProgress ProgressFunc) error {
	props, jar, err := c.fetchPropsFrom(ctx, pageURL)
	if err != nil {
		return err
	}

	// 途中まで送って中断する事態を避けるため、
	// 必要なファイルが揃っているかを先に確認する。
	var missing []string
	for _, t := range props.Targets {
		if _, ok := files[t.Key]; !ok {
			missing = append(missing, t.Key+t.FileExtension)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("アップロードするファイルがありません: %s", strings.Join(missing, ", "))
	}

	// ② 各ターゲットをオブジェクトストレージへ直接 PUT する。
	for _, t := range props.Targets {
		if err := c.putFile(ctx, t, files[t.Key], onProgress); err != nil {
			return err
		}
	}

	// ③ 完了を通知する。
	return c.notify(ctx, pageURL, props.CsrfToken, jar)
}

// putFile は②を行う。
func (c *Client) putFile(ctx context.Context, t Target, path string, onProgress ProgressFunc) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("ファイルを開けません (%s): %w", path, err)
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return fmt.Errorf("ファイル情報を取得できません (%s): %w", path, err)
	}

	var body io.Reader = f
	if onProgress != nil {
		body = &progressReader{r: f, total: st.Size(), key: t.Key, fn: onProgress}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, t.UploadURL, body)
	if err != nil {
		return err
	}
	req.ContentLength = st.Size()
	req.Header.Set("Content-Type", t.ContentType)
	if t.ContentEncoding != "" {
		req.Header.Set("Content-Encoding", t.ContentEncoding)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("アップロードに失敗しました (%s): %w", t.Key, err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// 署名付きURLには有効期限があるため、切れている可能性を伝える。
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return fmt.Errorf(
				"アップロードが拒否されました (%s, status=%d)。"+
					"署名付きURLの期限切れの可能性があります。再実行してください", t.Key, resp.StatusCode)
		}
		return fmt.Errorf("アップロードに失敗しました (%s, status=%d)", t.Key, resp.StatusCode)
	}
	return nil
}

// notify は③を行う。成功時は 302 が返る。
func (c *Client) notify(ctx context.Context, pageURL, csrfToken string, jar *auth.Jar) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, pageURL, nil)
	if err != nil {
		return err
	}
	// CSRFトークンはセッションと対で検証される。
	// ここで使うのは data-props の csrfToken で、
	// ページ内の input[name=authenticity_token] では 422 になる。
	req.Header.Set("Cookie", jar.Header())
	req.Header.Set("X-CSRF-Token", csrfToken)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Origin", BaseURL)
	req.Header.Set("Referer", pageURL)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("完了通知に失敗しました: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	// 2xx と 3xx（リダイレクト）が成功。
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return nil
	}
	if resp.StatusCode == http.StatusUnprocessableEntity {
		return fmt.Errorf("完了通知が拒否されました (422)。" +
			"セッションが切れている可能性があります。`ur-uploader login` を実行してください")
	}
	return fmt.Errorf("完了通知に失敗しました (status=%d)", resp.StatusCode)
}

// GameURL は公開ページのURLを返す。
func GameURL(permalink string) string {
	return fmt.Sprintf("%s/games/%s", BaseURL, permalink)
}

// progressReader は読み取り量を通知する Reader。
type progressReader struct {
	r     io.Reader
	total int64
	sent  int64
	key   string
	fn    ProgressFunc
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	if n > 0 {
		p.sent += int64(n)
		p.fn(p.key, p.sent, p.total)
	}
	return n, err
}
