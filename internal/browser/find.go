// Package browser は login のためにブラウザを起動し、CDP 経由で Cookie を取得する。
//
// ブラウザは同梱せず、利用者のマシンにある Chrome / Edge / Brave を使う。
// 独立した一時プロファイルで起動するため、普段使っているブラウザを
// 閉じる必要はなく、そちらのプロファイルにも影響しない。
package browser

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Browser は見つかったブラウザ。
type Browser struct {
	Name string
	Path string
}

// candidates は OS ごとの探索候補。Chrome を優先する。
func candidates() []Browser {
	switch runtime.GOOS {
	case "darwin":
		return []Browser{
			{"Google Chrome", "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"},
			{"Microsoft Edge", "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge"},
			{"Brave", "/Applications/Brave Browser.app/Contents/MacOS/Brave Browser"},
			{"Chromium", "/Applications/Chromium.app/Contents/MacOS/Chromium"},
		}

	case "windows":
		var dirs []string
		for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "LocalAppData"} {
			if v := os.Getenv(env); v != "" {
				dirs = append(dirs, v)
			}
		}
		var out []Browser
		for _, d := range dirs {
			out = append(out,
				Browser{"Google Chrome", filepath.Join(d, `Google\Chrome\Application\chrome.exe`)},
				Browser{"Microsoft Edge", filepath.Join(d, `Microsoft\Edge\Application\msedge.exe`)},
				Browser{"Brave", filepath.Join(d, `BraveSoftware\Brave-Browser\Application\brave.exe`)},
			)
		}
		return out

	default: // linux など
		return []Browser{
			{"Google Chrome", "google-chrome"},
			{"Google Chrome", "google-chrome-stable"},
			{"Chromium", "chromium"},
			{"Chromium", "chromium-browser"},
			{"Microsoft Edge", "microsoft-edge"},
			{"Brave", "brave-browser"},
		}
	}
}

// Find は利用可能なブラウザを探す。
// explicit が指定されていればそれを使う。
func Find(explicit string) (*Browser, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err == nil {
			return &Browser{Name: filepath.Base(explicit), Path: explicit}, nil
		}
		if p, err := exec.LookPath(explicit); err == nil {
			return &Browser{Name: filepath.Base(explicit), Path: p}, nil
		}
		return nil, fmt.Errorf("指定されたブラウザが見つかりません: %s", explicit)
	}

	for _, c := range candidates() {
		if filepath.IsAbs(c.Path) {
			if _, err := os.Stat(c.Path); err == nil {
				return &c, nil
			}
			continue
		}
		if p, err := exec.LookPath(c.Path); err == nil {
			return &Browser{Name: c.Name, Path: p}, nil
		}
	}

	return nil, fmt.Errorf(
		"Chrome / Edge / Brave のいずれも見つかりませんでした。\n" +
			"--browser でブラウザの実行ファイルを指定するか、\n" +
			"--manual で手動入力に切り替えてください")
}
