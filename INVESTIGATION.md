# unityroom 自動アップロード CLI 実現可能性調査

調査日: 2026-08-15 / 対象: 当初の構想（unityroom 自動アップロードCLI）

> 調査完了後、この結果に基づいて Go で実装した。
> 使い方は [README.md](README.md) を参照。

## 結論

**実現可能（実証済み）。ただし当初の技術選定（Playwright / TypeScript）は変更した。**

実際に Godot プロジェクトをビルドし、**ブラウザを使わず fetch のみで
unityroom へのアップロードを成功させた**:

```
① GET      200 | target: pck.pck
② PUT      201 | 1.6KB ✓
③ PATCH    302 ✓

=== アップロード状況 ===
Build.pck  1.63KB  2026年08月14日(金) 19時04分55秒
```

**アップロードしたゲームが unityroom 上で正常に起動することも確認済み。**
「Godotでビルド → コマンドでアップロード → ブラウザで動く」という
一連の流れが、エンドツーエンドで成立することを実証できた。

最大の発見は、**Playwright が deploy に不要**だったこと。アップロードは
署名付きURLへの直接 PUT で、素の HTTP リクエストで再現できる。
ブラウザなしの `fetch` で認証済みページを取得できることは**実証済み**。

ブラウザが要るのは初回の `login` だけ（OAuth のため）。

| 項目 | 判定 | 備考 |
|---|---|---|
| Godot ビルド → アップロード → 起動 | ✅ **実証済み** | E2Eで動作確認 |
| アップロードのHTTP化 | ✅ **実証済み** | fetchのみでGET/PUT/PATCH成功 |
| Godot headless ビルド | ✅ 実証済み | README記載のコマンドで成功 |
| エンジン自動判定 | ✅ 実現可能 | 低リスク |
| Unity WebGLビルド | ✅ 実現可能 | batchmode、低リスク |
| `login`（初回認証） | ⚠️ 条件付き | ブラウザ必須。Google要注意（下記） |
| セッション永続化 | ✅ 実現可能 | `remember_token` で永続。実証済み |
| ブラウザなしfetch | ✅ 実証済み | Cookieのみで200。UA偽装も不要 |
| Unity のアップロード仕様 | ❓ 未検証 | Godotのみ確認。複数ファイルの可能性 |
| 規約適合性 | ⚠️ グレー | 明示的禁止はないが下記留意 |

---

## 1. アップロードAPI仕様（実測・最重要）

アップロード画面は素の form ではなく、React 製の
`WebglFilesUploader-*.js` が制御している。だが通信自体は単純で、
**3ステップの HTTP リクエストのみ**。

### URL

```
https://unityroom.com/games/<permalink>/settings/webgl_upload
```

### 手順

**① GET でメタ情報を取得**

HTML 内の `div[data-props]` に全情報が入っている:

```json
{
  "targets": [{
    "key": "pck",
    "fileExtension": ".pck",
    "uploadUrl": "https://object-storage.tyo1.conoha.io/v1/nc_<account>/unityroom_production/game/<game_id>/webgl/Build.pck?temp_url_sig=<署名>&temp_url_expires=<UNIX時刻>",
    "contentType": "application/octet-stream"
  }],
  "uploadPath": "/games/<permalink>/settings/webgl_upload",
  "csrfToken": "...",
  "successMessageId": "success_message",
  "progressBarId": "progress_godot"
}
```

**② PUT でストレージに直接アップロード**

ConoHa オブジェクトストレージへ、署名付きURLに対して直接送信する。
unityroom のサーバーを経由しない。

```
PUT <uploadUrl>
Content-Type: application/octet-stream
（Content-Encoding: 指定があれば付与）
Body: Build.pck のバイナリ
```

**③ PATCH で完了通知**

```
PATCH /games/<permalink>/settings/webgl_upload
X-CSRF-Token: <csrfToken>
X-Requested-With: XMLHttpRequest
Cookie: <①のSet-Cookieで更新されたセッション>
```

成功時は **302** が返る（JS は `redirect: 'manual'` で呼び
`opaqueredirect` を成功扱いにしている）。実装でも**リダイレクトを追わない**こと。

### ⚠️ ここで必ずハマる（実測で解明した2点）

**1. ①の `Set-Cookie` を必ず引き継ぐ**

CSRF トークンは**セッションと対で**検証される。GET で
`_unity-room_session` が更新されるので、それを PATCH に引き継がないと
**422** になる。Cookie ジャーを持つか、`Set-Cookie` を反映すること。

**2. トークンは `data-props` のものを使う**

同じページに2種類のトークンがあり、**値が異なる**。

| 取得元 | PATCH での結果 |
|---|---|
| `data-props` の `csrfToken` | ✅ 302（成功） |
| `input[name=authenticity_token]` | ❌ 422 |

なお、セッションを引き継がずに送ると **503**（Application Error）に
なることもあった。422/503 いずれもこの2点が原因。

### ブラウザなしで動くことを実証済み

永続プロファイルから Cookie を取り出し、**Playwright を一切使わない
素の Node `fetch`** で①を実行した結果:

```
[Cookieのみ]   status=200  data-props=✓あり
  csrfToken: 取得成功(86文字)   targets: pck.pck   uploadUrl取得: ✓
[Cookie + UA]  status=200  data-props=✓あり
```

- **UA 偽装なしでも 200**。Cloudflare 配下だが弾かれない
- CSRF トークンと署名付きURLの両方が取得できた

### 意味すること

- **`deploy` に Playwright は不要**。`fetch` だけで実装できる（実証済み）
- 必要なのは `remember_token` などの永続 Cookie のみ
- 進捗は XHR の `progress` で取得している = HTTP でも取得可能
- 署名付きURLには**有効期限**がある（`temp_url_expires`）。
  GET から PUT までの間に取り直しが必要な場合がある

---

## 2. Godot / Unity のファイル形式

**Godot（画面で確認済み）**

> GodotでWebエクスポートしたときに生成される **Build.pck** ファイルを
> アップロードしてください。

- 単一ファイル
- `input[type=file]` は `multiple: true` だが Godot は pck 1つ
- **.NET 版は Web エクスポート非対応**（公式ヘルプに明記）

**❗ 出力される pck の名前に注意（実測）**

当初の想定通り、headless エクスポートは成功する:

```bash
godot --headless --path . --export-release "Web" dist/index.html
```

しかし出力される pck は **`index.pck`**（エクスポート先の basename に従う）で、
unityroom が要求する **`Build.pck`** とは名前が違う。

**→ 実装では、エクスポート先を `dist/Build.html` にすることで
`Build.pck` を直接生成させ、リネームを不要にした**（`internal/engine/godot.go`）。

- なお `Build.wasm`（約38MB）等も生成されるが、**unityroom に渡すのは pck のみ**。
  エンジン本体はサービス側が持っている
- 画面は**エンジン別に出し分けられている**（「Webビルドアップロード (Godot)」）。
  別途「Webビルド設定」でエンジンを指定していると推測される

**GDExtension（別画面・部分的に調査済み）**

`/games/<permalink>/settings/gdextensions` に専用画面があり、
`GdextensionsUploader-*.js` が制御している。WebGL とは別系統。

- 対象は **`.so` / `.wasm`**、複数ファイル可
- パスに `^[a-zA-Z0-9()_./-]+$` のバリデーションがある
- 属性名が `data-props` ではなく **`data`**
- `purge_cache` があり、上書き時は実行が必要とされている

詳細は [#2](https://github.com/KOU050223/ur-uploader/issues/2)。

**Unity（未検証）**

`multiple: true` かつ `targets` が配列であることから、
Unity は複数ファイル（`.wasm`/`.data`/`.js`/`.json`）を
それぞれ PUT する構造と推測される。**要検証**。

なお外部情報では、unityroom 向けは**圧縮形式を Gzip にする必要がある**とされる
（Brotli では起動しない）。公式一次情報での裏取りは未実施。

---

## 3. 認証（最大の注意点）

### ログイン方式

OAuth のみ（**Twitter / Google / GitHub**）。ID/パスワード入力欄は存在しない。
そのため **`login` はブラウザ必須**（OAuth は人間の操作が要る）。

### Google は自動化ブラウザを検出してブロックする（実測）

Playwright 同梱 Chromium で開くと、Google が拒否する:

> このブラウザまたはアプリは安全でない可能性があります。

**回避策（実証済み）**: 以下で `navigator.webdriver = false` となり通過できる。

```ts
chromium.launchPersistentContext(dir, {
  args: ['--disable-blink-features=AutomationControlled'],
  ignoreDefaultArgs: ['--enable-automation'],
})
```

- 実 Chrome（`channel: 'chrome'`）は**普段の Chrome 起動中に使えない**
  （「既存のブラウザ セッションで開いています」で落ちる）ため不採用
- **GitHub ログインは問題なく通過**（実測）

**最終的な解決策（実装済み・実証済み）**

Playwright を使わず、**CDP でユーザーのブラウザを直接操作**する方式にした。

```
--user-data-dir=<一時ディレクトリ>          ← 普段のブラウザと分離（起動中でも共存可）
--remote-debugging-port=<空きポート>        ← CDP で Cookie を取得
--disable-blink-features=AutomationControlled  ← 自動化検出の回避
```

一時プロファイルを使うことで「既存セッションで開いています」問題も回避できる。
`internal/browser/` に実装済みで、**ログイン検知から保存まで自動化されている**。

### ✅ セッション永続化は可能（実証済み）

**「初回だけ login、以降は使い回す」は成立する。**

重要なのは**認証の実体がどの Cookie か**を取り違えないこと。

| Cookie | 性質 | 役割 |
|---|---|---|
| `remember_token` | 永続 | **これが認証の実体** |
| `user_id` / `is_member` | 永続 | 併せて保持する |
| `_unity-room_session` | `expires: -1` | アクセス時に発行される一時Cookie。**保存不要** |

調査中、`_unity-room_session` だけを保存して復元に失敗し
「セッションが短命」と誤認したが、原因は**保存すべき Cookie が
違っていた**だけだった。`remember_token` を保持すれば問題なく復元できる。

実証内容:
- 永続プロファイルが**4プロセスを跨いで**認証を維持（headless 含む）
- そのプロファイルから Cookie を取り出し、**素の `fetch` で 200 応答**

**ログイン完了判定の注意**: `a[href="/login"]` の有無で判定すると
誤検知する（OAuth 遷移中に消えるため）。
`form[action="/logout"]` の存在など**肯定的なシグナル**で判定すべき。

---

## 4. 規約・技術的前提

**規約**（[unityroom.com/terms](https://unityroom.com/terms) 全文確認、最終更新 2021-12-12）

- **自動化・スクレイピングの明示的な禁止条項はない**
- ただし第5条に以下があり、**グレーゾーン**である点は明記すべき:
  - 「サーバーまたはネットワークの機能を破壊したり、妨害したりする行為」
  - 「本サービスが予定している利用目的と異なる目的で本サービスを利用する行為」
- robots.txt は `Allow: /` だが、これはクロール許可であり
  **認証済みの書き込み操作を許可するものではない**

推奨する配慮:
- 並列アップロードをしない、429 を尊重する
- 自分のゲームのみを対象にする
- **非公式ツールであることを明記**する
- ツールがログイン中ユーザーとして動作することを明示する

**その他**

- Godot 対応は 2026-04-04 から（GDExtension は 2026-04-29 から）で事実
- ゲームURL: `/games/<permalink>`、管理画面: `/games/<permalink>/settings/*`
- 新規作成: POST `/games`（`game[title]`, `game[permalink]`,
  `game[introduction]`, `game[estimated_playtime]`）
- **先行事例は発見できなかった**（競合なし／参考実装もなし）

---

## 5. 推奨アーキテクチャ

当初構想の「全部 Playwright」から変更した。

```
ur-uploader login    → ブラウザ（OAuth のため不可避）
                        Cookie を取得して保存
ur-uploader deploy   → ビルド → fetch のみ（ブラウザ不要）
```

**利点**: CI で動く / 高速 / 軽量 / Google のブロック問題と無縁 /
DOM 変更に強い（data-props は DOM より安定）

**留意**: 非公開APIのため予告なく変わりうる。
`data-props` の形が変わったら明示的にエラーを出す設計にする。

### 実装での選択（この方針で実装済み）

言語は **Go** を採用した。単一バイナリで配布でき、
利用者に Node などのランタイムを要求しないため。

`login` のブラウザ操作も Playwright を使わず、
**CDP でユーザーのブラウザを直接操作**する方式にした
（`internal/browser/`）。ブラウザを同梱しないためバイナリは 7MB 弱で済む。

ブラウザが無い環境向けに `--manual`（Cookie を手入力）も用意した。

---

## 6. 次にやるべきこと（優先順）

Godot 経路はエンドツーエンドで実証済み。**MVP の実装に着手できる状態**。

1. **Unity のアップロード仕様の確認**（Unity対応をやるなら最優先）
   - 「Webビルド設定」でエンジンを Unity に切り替えて `data-props` を見る
   - `targets` が複数になるか、`contentEncoding` に何を要求するか
     （Gzip/Brotli 問題の一次情報が得られる）
   - Godot と同じ3ステップで済むかを確認
2. `remember_token` の有効期間の確認（期限切れ検知の設計に必要）
3. 大容量ファイルでの挙動（Unity WebGL は100MB超もありうる。
   署名付きURLの有効期限内に送りきれるか）
4. Google ログインの可否を現構成で再確認（GitHubで代替可能なため優先度低）

---

## 付録: 調査結果の実装先

調査で判明した内容は、以下に実装されている。
検証に使った JS のプロトタイプは役目を終えたため削除した。

| 調査項目 | 実装先 |
|---|---|
| GET → PUT → PATCH の3ステップ | `internal/unityroom/client.go` |
| Cookieジャー（セッション引き継ぎ） | `internal/auth/auth.go` |
| CDP でのブラウザ操作 | `internal/browser/` |
| Godot の headless ビルド | `internal/engine/godot.go` |

**注記**: 調査中の実アップロードは**非公開のテスト用ゲーム**に対してのみ
実施した。公開設定の変更や、他のゲームへの操作は一切行っていない。
