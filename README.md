# ur-uploader

**unityroom へのゲームアップロードを自動化する非公式CLIツール**

```bash
$ ur-uploader deploy my-game

✓ godot プロジェクトを検出しました
✓ ビルド完了 (2.3s)
✓ アップロード完了

https://unityroom.com/games/my-game
```

> [!IMPORTANT]
> これは**非公式ツール**です。unityroom 運営とは関係ありません。
> 公式APIが提供されていないため、Web UI が内部で使っているエンドポイントを
> 利用しています。予告なく動作しなくなる可能性があります。

## できること

- **Godot** プロジェクトの Web ビルド → unityroom へのアップロードを1コマンドで
- 単一バイナリで動作（ランタイム不要）
- エンジンの自動判定

Unity は**ビルドまで対応**。アップロードは仕様調査が済んでいないため未対応です
（[INVESTIGATION.md](INVESTIGATION.md) 参照）。

## インストール

### バイナリをダウンロード

[Releases](../../releases) から各OS向けのバイナリを取得してください。

### go install

```bash
go install github.com/KOU050223/ur-uploader@latest
```

## 使い方

### 1. ログイン（初回のみ）

```bash
ur-uploader login
```

ブラウザが開くので unityroom にログインしてください。
ログインを検知すると自動的に認証情報を保存し、ブラウザを閉じます。

認証情報は `~/.ur-uploader/auth.json` に保存されます（パーミッション 0600）。

> [!NOTE]
> ブラウザは同梱していません。お使いの **Chrome / Edge / Brave** を使います。
> 普段のブラウザとは別の一時プロファイルで起動するため、
> 起動中のブラウザを閉じる必要はなく、そちらには影響しません。
> 一時プロファイルは終了時に削除されます。

ブラウザが見つからない場合や、自動起動を使いたくない場合は手動入力も使えます。

```bash
ur-uploader login --manual   # Cookie を自分で貼り付ける
ur-uploader login --browser /path/to/chrome   # ブラウザを明示指定
```

### 2. デプロイ

```bash
# プロジェクトのディレクトリで
ur-uploader deploy <game-id>
```

`game-id` は unityroom のゲームURLの末尾部分です。

```
https://unityroom.com/games/my-game
                            ^^^^^^^ これ
```

## コマンド

| コマンド | 説明 |
|---|---|
| `login` | 認証情報を保存する（初回のみ） |
| `deploy <game-id>` | ビルドしてアップロード |
| `build` | ビルドのみ |
| `upload <game-id> --pck <file>` | 既存の成果物をアップロード |

### 主なオプション

```bash
-C, --dir <path>        プロジェクトのディレクトリ（既定: .）
    --engine <name>     エンジンを明示指定 (godot|unity)
    --no-build          ビルドせず既存の成果物を使う
    --pck <file>        アップロードするファイルを直接指定
-o, --output <dir>      ビルド出力先（既定: dist）
    --preset <name>     Godot のエクスポートプリセット名（既定: Web）
-v, --verbose           エンジンの出力を表示
```

## 事前準備

### Godot

エクスポートプリセットに **Web** 向けの設定が必要です。
Godot エディタの「プロジェクト → エクスポート」から追加してください。

`export_presets.cfg` がコミットされていれば CI でも動きます。

> [!NOTE]
> Godot の **.NET版は Web エクスポートに非対応**です（unityroom 公式ヘルプに記載）。

## 仕組み

```
エンジン判定 → Web ビルド → unityroom へアップロード
```

アップロードは以下の3ステップで行われます。

1. **GET** アップロード画面から署名付きURLとCSRFトークンを取得
2. **PUT** オブジェクトストレージへ直接アップロード
3. **PATCH** unityroom へ完了を通知

`deploy` はブラウザを使わず HTTP のみで動作するため、軽量で CI でも動きます。
ブラウザを使うのは `login` のときだけです（OAuth ログインのため）。
その際も同梱はせず、CDP 経由でお使いのブラウザを操作します。

詳細は [INVESTIGATION.md](INVESTIGATION.md) を参照してください。

## 注意事項

- **自分のゲームに対してのみ**使用してください
- unityroom の[利用規約](https://unityroom.com/terms)に従ってください
- サーバーに負荷をかける使い方（連続実行、並列アップロード）は避けてください

## ライセンス

MIT
