package browser

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// LoginURL は unityroom のログイン画面。
const LoginURL = "https://unityroom.com/login"

// WantedCookies は保存する Cookie 名。
// remember_token が認証の実体で、他は補助的に保持する。
var WantedCookies = []string{"remember_token", "user_id", "is_member"}

// requiredCookie がそろえばログイン完了とみなす。
const requiredCookie = "remember_token"

// WaitForLogin はブラウザでのログイン完了を待ち、Cookie を返す。
//
// onWait は待機中に定期的に呼ばれる（経過秒数を渡す）。
func WaitForLogin(ctx context.Context, s *Session, timeout time.Duration, onWait func(elapsed time.Duration)) ([]Cookie, error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}

		if !s.Alive() {
			return nil, fmt.Errorf("ブラウザが閉じられました")
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("ログインがタイムアウトしました")
		}

		cookies, err := s.Cookies(ctx)
		if err != nil {
			// 一時的な失敗は無視して再試行する。
			continue
		}

		if found := pick(cookies); found != nil {
			return found, nil
		}

		if onWait != nil {
			onWait(time.Since(start))
		}
	}
}

// pick は unityroom の必要な Cookie がそろっていれば返す。
func pick(cookies []Cookie) []Cookie {
	var out []Cookie
	ok := false

	for _, c := range cookies {
		if !strings.Contains(c.Domain, "unityroom.com") {
			continue
		}
		for _, want := range WantedCookies {
			if c.Name != want || c.Value == "" {
				continue
			}
			out = append(out, c)
			if c.Name == requiredCookie {
				ok = true
			}
		}
	}

	if !ok {
		return nil
	}
	return out
}
