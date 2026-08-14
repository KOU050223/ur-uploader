package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/kou050223/ur-uploader/internal/auth"
	"github.com/kou050223/ur-uploader/internal/browser"
	"github.com/spf13/cobra"
)

func newLoginCommand() *cobra.Command {
	var (
		manual      bool
		browserPath string
		timeout     time.Duration
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "unityroom にログインして認証情報を保存する",
		Long: `unityroom の認証情報を保存します。

ブラウザが開くので、unityroom にログインしてください。
ログインを検知すると自動的に認証情報を保存し、ブラウザを閉じます。

保存は初回の一度だけで済みます。`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if manual {
				return runManualLogin()
			}
			err := runBrowserLogin(cmd.Context(), browserPath, timeout)
			if err != nil {
				// ブラウザが見つからない場合などは手動方式を案内する。
				fmt.Fprintln(os.Stderr)
				fmt.Fprintln(os.Stderr, "エラー:", err)
				fmt.Fprintln(os.Stderr)
				fmt.Fprintln(os.Stderr, "`ur-uploader login --manual` で手動入力に切り替えられます。")
				os.Exit(1)
			}
			return nil
		},
	}

	fl := cmd.Flags()
	fl.BoolVar(&manual, "manual", false, "ブラウザを使わず Cookie を手入力する")
	fl.StringVar(&browserPath, "browser", "", "使用するブラウザの実行ファイル")
	fl.DurationVar(&timeout, "timeout", 5*time.Minute, "ログイン待機のタイムアウト")
	return cmd
}

// runBrowserLogin はブラウザを起動し、ログイン完了を検知して保存する。
func runBrowserLogin(ctx context.Context, browserPath string, timeout time.Duration) error {
	b, err := browser.Find(browserPath)
	if err != nil {
		return err
	}
	fmt.Printf("✓ ブラウザ: %s\n", b.Name)

	sess, err := browser.Launch(ctx, b, browser.LoginURL)
	if err != nil {
		return err
	}
	defer sess.Close()

	fmt.Println()
	fmt.Println("ブラウザで unityroom にログインしてください。")
	fmt.Println("（このウィンドウは普段のブラウザとは別のプロファイルで動いています）")
	fmt.Println()

	last := -1
	cookies, err := browser.WaitForLogin(ctx, sess, timeout, func(elapsed time.Duration) {
		if s := int(elapsed.Seconds()); s != last {
			last = s
			progressf("  ログイン待機中... %ds", s)
		}
	})
	clearLine()
	if err != nil {
		return err
	}

	creds := &auth.Credentials{}
	for _, c := range cookies {
		creds.Cookies = append(creds.Cookies, auth.Cookie{Name: c.Name, Value: c.Value})
	}
	if err := auth.Save(creds); err != nil {
		return err
	}

	path, _ := auth.Path()
	fmt.Println("✓ ログインを検知しました")
	fmt.Printf("✓ 認証情報を保存しました: %s\n", path)
	return nil
}

// runManualLogin はブラウザ制御を使わず、Cookie を手入力してもらう。
func runManualLogin() error {
	fmt.Println("unityroom にログインします。")
	fmt.Println()

	if err := openBrowser(browser.LoginURL); err != nil {
		fmt.Printf("ブラウザで開いてください: %s\n", browser.LoginURL)
	} else {
		fmt.Printf("ブラウザで開きました: %s\n", browser.LoginURL)
	}

	fmt.Println()
	fmt.Println("ログイン後、以下の手順で remember_token を取得してください:")
	fmt.Println()
	fmt.Println("  1. F12 で開発者ツールを開く")
	fmt.Println("  2. Application（アプリケーション）タブ → Cookies → https://unityroom.com")
	fmt.Println("  3. remember_token の Value をコピー")
	fmt.Println()
	fmt.Print("remember_token を貼り付けてください: ")

	token, err := readLine()
	if err != nil {
		return err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("入力が空です")
	}
	// 誤って "remember_token=xxx" の形で貼られることを想定して吸収する。
	if _, v, ok := strings.Cut(token, "="); ok {
		token = strings.TrimSpace(v)
	}

	creds := &auth.Credentials{
		Cookies: []auth.Cookie{{Name: auth.RequiredCookie, Value: token}},
	}
	if err := auth.Save(creds); err != nil {
		return err
	}

	path, _ := auth.Path()
	fmt.Println()
	fmt.Printf("✓ 認証情報を保存しました: %s\n", path)
	return nil
}

func readLine() (string, error) {
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("入力を読み取れません: %w", err)
	}
	return line, nil
}

// openBrowser は既定のブラウザで URL を開く。
func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}
	return exec.Command(cmd, args...).Start()
}
