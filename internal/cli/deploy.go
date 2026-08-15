package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/KOU050223/ur-uploader/internal/auth"
	"github.com/KOU050223/ur-uploader/internal/engine"
	"github.com/KOU050223/ur-uploader/internal/unityroom"
	"github.com/spf13/cobra"
)

// buildFlags はビルド関連の共通フラグ。
type buildFlags struct {
	dir         string
	engineName  string
	godotPath   string
	unityPath   string
	outputDir   string
	preset      string
	buildMethod string
	verbose     bool
}

func (f *buildFlags) register(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.StringVarP(&f.dir, "dir", "C", ".", "プロジェクトのディレクトリ")
	fl.StringVar(&f.engineName, "engine", "", "エンジンを明示指定 (godot|unity)")
	fl.StringVar(&f.godotPath, "godot-path", "", "Godot 実行ファイルのパス")
	fl.StringVar(&f.unityPath, "unity-path", "", "Unity 実行ファイルのパス")
	fl.StringVarP(&f.outputDir, "output", "o", "", "ビルド出力先 (既定: export_presets.cfg の設定に従う)")
	fl.StringVar(&f.preset, "preset", "", "Godot のエクスポートプリセット名 (既定: Web 向けのプリセットを自動選択)")
	fl.StringVar(&f.buildMethod, "build-method", "", "Unity の -executeMethod に渡す値")
	fl.BoolVarP(&f.verbose, "verbose", "v", false, "エンジンの出力を表示する")
}

// resolve はエンジンとビルド設定を確定する。
func (f *buildFlags) resolve() (engine.Engine, string, engine.BuildOptions, error) {
	dir, err := filepath.Abs(f.dir)
	if err != nil {
		return nil, "", engine.BuildOptions{}, err
	}

	var eng engine.Engine
	if f.engineName != "" {
		eng, err = engine.ByName(f.engineName)
	} else {
		eng, err = engine.Detect(dir)
	}
	if err != nil {
		return nil, "", engine.BuildOptions{}, err
	}

	execPath := f.godotPath
	if eng.Name() == "unity" {
		execPath = f.unityPath
	}

	return eng, dir, engine.BuildOptions{
		ExecPath:    execPath,
		OutputDir:   f.outputDir,
		Preset:      f.preset,
		BuildMethod: f.buildMethod,
		Verbose:     f.verbose,
	}, nil
}

func newDeployCommand() *cobra.Command {
	var f buildFlags
	var noBuild bool
	var pckPath string

	cmd := &cobra.Command{
		Use:   "deploy <game-id>",
		Short: "ビルドして unityroom にアップロードする",
		Long: `プロジェクトをビルドし、unityroom にアップロードします。

game-id は unityroom のゲームURLの末尾部分です。
  https://unityroom.com/games/my-game  ->  my-game`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDeploy(cmd.Context(), args[0], &f, noBuild, pckPath)
		},
	}

	f.register(cmd)
	cmd.Flags().BoolVar(&noBuild, "no-build", false, "ビルドせず既存の成果物を使う")
	cmd.Flags().StringVar(&pckPath, "pck", "", "アップロードする pck ファイルを直接指定する")
	return cmd
}

func runDeploy(ctx context.Context, permalink string, f *buildFlags, noBuild bool, pckPath string) error {
	// 先に認証を確認する。ビルド後に落ちると時間を無駄にするため。
	creds, err := auth.Load()
	if err != nil {
		return err
	}

	files := map[string]string{}

	switch {
	case pckPath != "":
		st, err := os.Stat(pckPath)
		if err != nil {
			return fmt.Errorf("指定されたファイルが見つかりません: %s", pckPath)
		}
		if !st.Mode().IsRegular() {
			return fmt.Errorf("ファイルではありません: %s", pckPath)
		}
		files["pck"] = pckPath
		fmt.Printf("✓ アップロードするファイル: %s\n", pckPath)

	case noBuild:
		eng, dir, opts, err := f.resolve()
		if err != nil {
			return err
		}
		// --no-build は Godot の成果物を前提にしている。
		if eng.Name() != "godot" {
			return fmt.Errorf(
				"--no-build は現在 Godot のみ対応しています。--pck で指定してください")
		}
		// ビルド時と同じ規則（export_presets.cfg 準拠）で成果物を探す。
		_, pck, err := engine.ResolveGodotPck(dir, opts.Preset, opts.OutputDir)
		if err != nil {
			return err
		}
		if _, err := os.Stat(pck); err != nil {
			return fmt.Errorf(
				"ビルド成果物が見つかりません: %s\n--no-build を外すか --pck で指定してください", pck)
		}
		files["pck"] = pck
		fmt.Printf("✓ %s プロジェクト（ビルドをスキップ）\n", eng.Name())

	default:
		eng, dir, opts, err := f.resolve()
		if err != nil {
			return err
		}
		fmt.Printf("✓ %s プロジェクトを検出しました\n", eng.Name())

		progressf("  ビルド中...")
		start := time.Now()
		artifact, err := eng.Build(ctx, dir, opts)
		if err != nil {
			fmt.Println()
			return err
		}
		donef("✓ ビルド完了 (%.1fs)\n", time.Since(start).Seconds())
		files = artifact.Files
	}

	// アップロード
	client := unityroom.New(creds)
	lastPct := -1
	err = client.Upload(ctx, permalink, files, func(key string, sent, total int64) {
		if total <= 0 {
			return
		}
		pct := int(sent * 100 / total)
		if pct != lastPct {
			lastPct = pct
			progressf("  アップロード中... %d%%", pct)
		}
	})
	if err != nil {
		clearLine()
		return err
	}

	donef("✓ アップロード完了\n\n")
	fmt.Println(unityroom.GameURL(permalink))
	return nil
}

func newBuildCommand() *cobra.Command {
	var f buildFlags

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Web 向けビルドのみを行う",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, dir, opts, err := f.resolve()
			if err != nil {
				return err
			}
			fmt.Printf("✓ %s プロジェクトを検出しました\n", eng.Name())

			progressf("  ビルド中...")
			start := time.Now()
			artifact, err := eng.Build(cmd.Context(), dir, opts)
			if err != nil {
				fmt.Println()
				return err
			}
			donef("✓ ビルド完了 (%.1fs)\n", time.Since(start).Seconds())
			for key, path := range artifact.Files {
				fmt.Printf("  %s: %s\n", key, path)
			}
			return nil
		},
	}

	f.register(cmd)
	return cmd
}

func newUploadCommand() *cobra.Command {
	var pckPath string

	cmd := &cobra.Command{
		Use:   "upload <game-id>",
		Short: "既存のビルド成果物をアップロードする",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if pckPath == "" {
				return fmt.Errorf("--pck でアップロードするファイルを指定してください")
			}
			f := &buildFlags{dir: "."}
			return runDeploy(cmd.Context(), args[0], f, false, pckPath)
		},
	}

	cmd.Flags().StringVar(&pckPath, "pck", "", "アップロードする pck ファイル")
	return cmd
}
