package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/coder/websocket"
)

// Cookie は CDP から取得した Cookie。
type Cookie struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
}

// Session は起動中のブラウザとの接続。
type Session struct {
	cmd     *exec.Cmd
	conn    *websocket.Conn
	profile string
	nextID  int
}

// Launch はブラウザを一時プロファイルで起動し、CDP に接続する。
// 呼び出し側は必ず Close を呼ぶこと。
func Launch(ctx context.Context, b *Browser, startURL string) (*Session, error) {
	port, err := freePort()
	if err != nil {
		return nil, fmt.Errorf("空きポートを確保できません: %w", err)
	}

	// 普段のブラウザと分離するため一時プロファイルを使う。
	// これがないと起動中のブラウザに委譲されて制御できない。
	profile, err := os.MkdirTemp("", "ur-uploader-")
	if err != nil {
		return nil, fmt.Errorf("一時プロファイルを作成できません: %w", err)
	}

	cmd := exec.Command(b.Path,
		"--user-data-dir="+profile,
		fmt.Sprintf("--remote-debugging-port=%d", port),
		"--no-first-run",
		"--no-default-browser-check",
		// navigator.webdriver を無効化する。
		// これがないと Google のログイン画面が
		// 「このブラウザまたはアプリは安全でない可能性があります」として拒否し、
		// 利用者が自分のアカウントにログインできない。
		// ログイン操作自体は利用者本人が手動で行う（自動入力はしない）。
		"--disable-blink-features=AutomationControlled",
		startURL,
	)
	if err := cmd.Start(); err != nil {
		os.RemoveAll(profile)
		return nil, fmt.Errorf("ブラウザを起動できません: %w", err)
	}

	s := &Session{cmd: cmd, profile: profile}

	wsURL, err := waitForDebugger(ctx, port)
	if err != nil {
		s.Close()
		return nil, err
	}

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		s.Close()
		return nil, fmt.Errorf("ブラウザに接続できません: %w", err)
	}
	// CDP のメッセージは大きくなりうるので上限を上げる。
	conn.SetReadLimit(32 << 20)
	s.conn = conn

	return s, nil
}

// Close はブラウザを終了し、一時プロファイルを削除する。
func (s *Session) Close() {
	if s.conn != nil {
		s.conn.Close(websocket.StatusNormalClosure, "")
	}
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
		s.cmd.Wait()
	}
	if s.profile != "" {
		os.RemoveAll(s.profile)
	}
}

// cdpResponse は CDP の応答。
type cdpResponse struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// call は CDP のメソッドを呼ぶ。
func (s *Session) call(ctx context.Context, method string, result any) error {
	s.nextID++
	id := s.nextID

	req, err := json.Marshal(map[string]any{
		"id":     id,
		"method": method,
		"params": map[string]any{},
	})
	if err != nil {
		return err
	}
	if err := s.conn.Write(ctx, websocket.MessageText, req); err != nil {
		return fmt.Errorf("ブラウザへの送信に失敗しました: %w", err)
	}

	// 自分の id の応答が来るまで読み飛ばす（イベントが混ざるため）。
	for {
		_, data, err := s.conn.Read(ctx)
		if err != nil {
			return fmt.Errorf("ブラウザからの受信に失敗しました: %w", err)
		}
		var resp cdpResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			continue
		}
		if resp.ID != id {
			continue
		}
		if resp.Error != nil {
			return fmt.Errorf("ブラウザ操作に失敗しました: %s", resp.Error.Message)
		}
		if result != nil {
			return json.Unmarshal(resp.Result, result)
		}
		return nil
	}
}

// Cookies は現在の Cookie を取得する。
func (s *Session) Cookies(ctx context.Context) ([]Cookie, error) {
	var out struct {
		Cookies []Cookie `json:"cookies"`
	}
	if err := s.call(ctx, "Storage.getCookies", &out); err != nil {
		return nil, err
	}
	return out.Cookies, nil
}

// Alive はブラウザがまだ動いているかを返す。
// 利用者がウィンドウを閉じた場合に待機を打ち切るために使う。
func (s *Session) Alive() bool {
	if s.cmd == nil || s.cmd.Process == nil {
		return false
	}
	return processAlive(s.cmd.Process)
}

// waitForDebugger は CDP が待ち受けるまで待ち、WebSocket URL を返す。
func waitForDebugger(ctx context.Context, port int) (string, error) {
	url := fmt.Sprintf("http://127.0.0.1:%d/json/version", port)
	client := &http.Client{Timeout: 2 * time.Second}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		resp, err := client.Get(url)
		if err == nil {
			var v struct {
				WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
			}
			err = json.NewDecoder(resp.Body).Decode(&v)
			resp.Body.Close()
			if err == nil && v.WebSocketDebuggerURL != "" {
				return v.WebSocketDebuggerURL, nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return "", fmt.Errorf("ブラウザの起動を確認できませんでした")
}

// freePort は空きポートを1つ確保する。
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
