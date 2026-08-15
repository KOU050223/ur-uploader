package browser

import "testing"

func TestIsUnityroomDomain(t *testing.T) {
	tests := []struct {
		domain string
		want   bool
	}{
		{"unityroom.com", true},
		{".unityroom.com", true},
		{"www.unityroom.com", true},
		{".www.unityroom.com", true},

		// 認証Cookieを誤って渡してはいけないもの
		{"evil-unityroom.com", false},
		{"unityroom.com.attacker.net", false},
		{"notunityroom.com", false},
		{"example.com", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := isUnityroomDomain(tt.domain); got != tt.want {
			t.Errorf("isUnityroomDomain(%q) = %v, want %v", tt.domain, got, tt.want)
		}
	}
}

func TestPick(t *testing.T) {
	t.Run("必要なCookieが揃えば返す", func(t *testing.T) {
		got := pick([]Cookie{
			{Name: "remember_token", Value: "tok", Domain: ".unityroom.com"},
			{Name: "user_id", Value: "1", Domain: ".unityroom.com"},
			{Name: "_ga", Value: "x", Domain: ".unityroom.com"}, // 保存対象外
		})
		if len(got) != 2 {
			t.Fatalf("件数 = %d, want 2 (%+v)", len(got), got)
		}
	})

	t.Run("remember_tokenが無ければnil", func(t *testing.T) {
		if got := pick([]Cookie{
			{Name: "user_id", Value: "1", Domain: ".unityroom.com"},
		}); got != nil {
			t.Errorf("nil を期待したが: %+v", got)
		}
	})

	t.Run("別ドメインのCookieは拾わない", func(t *testing.T) {
		if got := pick([]Cookie{
			{Name: "remember_token", Value: "tok", Domain: "evil-unityroom.com"},
		}); got != nil {
			t.Errorf("nil を期待したが: %+v", got)
		}
	})

	t.Run("値が空なら無効", func(t *testing.T) {
		if got := pick([]Cookie{
			{Name: "remember_token", Value: "", Domain: ".unityroom.com"},
		}); got != nil {
			t.Errorf("nil を期待したが: %+v", got)
		}
	})
}
