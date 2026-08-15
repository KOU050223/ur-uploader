**unityroom へのゲームアップロードを自動化する非公式CLIツールです。**

```bash
ur-uploader login              # 初回のみ
ur-uploader deploy my-game     # ビルドしてアップロード
```

```
✓ godot プロジェクトを検出しました
✓ ビルド完了 (2.3s)
✓ アップロード完了

https://unityroom.com/games/my-game
```

> [!IMPORTANT]
> これは**非公式ツール**です。unityroom 運営とは関係ありません。
> 公式APIが提供されていないため、Web UI が内部で使っているエンドポイントを
> 利用しています。**予告なく動作しなくなる可能性があります。**

## インストール

下の Assets からお使いの環境のファイルをダウンロードし、
展開して PATH の通った場所に置いてください。

| OS | ファイル |
|---|---|
| macOS (Apple Silicon) | `darwin_arm64` |
| macOS (Intel) | `darwin_amd64` |
| Windows | `windows_amd64` |
| Linux | `linux_amd64` / `linux_arm64` |

Go をお使いなら以下でも入ります。

```bash
go install github.com/KOU050223/ur-uploader@latest
```

## 特徴

- **単一バイナリ**。Node などのランタイムは不要です
- **`deploy` はブラウザを使いません**。HTTPのみで動くため軽量で、CI でも動きます
- `login` のみブラウザを使いますが、**同梱していません**。
  お使いの Chrome / Edge / Brave を一時プロファイルで起動するため、
  普段のブラウザを閉じる必要はありません

## 既知の制限

- **Godot のみ対応**です。Unity はビルドまで動きますが、
  アップロードは仕様調査が済んでいないため未対応です
- Godot の **.NET版は Web エクスポート非対応**のため使えません
  （unityroom 公式ヘルプに記載）
- Google アカウントでのログインが弾かれる場合は、
  GitHub や X(Twitter) でのログインをお試しください

## 注意

- **自分のゲームに対してのみ**使用してください
- unityroom の[利用規約](https://unityroom.com/terms)に従ってください
- サーバーに負荷をかける使い方（連続実行、並列アップロード）は避けてください

---

技術的な調査結果は [INVESTIGATION.md](https://github.com/KOU050223/ur-uploader/blob/main/INVESTIGATION.md) に、
使い方は [README](https://github.com/KOU050223/ur-uploader/blob/main/README.md) にまとめています。
