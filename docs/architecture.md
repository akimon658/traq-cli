# アーキテクチャ

traq-cli のパッケージ構成と設計意図を定める。責務ごとにパッケージを分け、どこに何を書くかを一意にする。

## パッケージ構成

```
cmd/traq/              エントリポイント。cli.Execute() を呼ぶだけ
internal/cli/          cobra の配線のみ
internal/config/       実行設定 Env と既定値解決 (TRAQ_* 環境変数)
internal/session/      トークン解決・401時の refresh リトライ・スコープ事前検査
internal/auth/         OAuth プリミティブ (PKCE / 認可URL / トークン交換 / Token / Store / SessionStore)
internal/authflow/     認証の手順 (login / manual / complete / logout / status 検証)
internal/client/       go-traq の薄ラッパ。origin とトークン注入を隠す
internal/channel/      チャンネル解決・未読取得・分類
internal/file/         ファイル取得 (ダウンロード) と保存
internal/message/      メッセージ取得・検索・投稿と付随情報の拡充
internal/user/         自ユーザー取得
internal/view/         人間可読テキストの出力専任。API を呼ばない
internal/markdown/     埋め込み解決の純粋パーサ。副作用なし
internal/exit/         終了コードと ExitCodeError
```

依存方向は次の通り。下位は上位を知らない。

```
cli → {authflow, channel, file, message, user} → session → config → auth
cli → view → {channel, file, message, auth, markdown}
message → channel, markdown, user
session → client, auth, exit
```

規約:

- `cli` はフラグ検証 (usage 系) → `session.ResolveToken` → ドメインサービス → `view` の順に呼び出す配線であり、API 呼び出しや整形ロジックは持たない
- ドメインサービス (`channel` / `message` / `user` / `authflow`) はデータを返し、表示は `view` に任せる。`view` を import しない
- `view` は `io.Writer` への出力だけを行い、API を呼ばない。入力型は `message` / `channel` / `auth` の型を参照する
- `session.Exec` が唯一の API 実行経路。401 時の refresh・保存・リトライはここに集約されており、呼び出し元は通常の呼び出しと同じ形で使う
- 終了コードと `exit.Error` は `exit` が定義する。`session` やドメインサービスはこれを返し、`cli.Execute` がコードに変換する

## 実行フロー

```
config → session → ドメインサービス (session.Exec 経由) → view
```

トークンの解決順・スコープ検査・401 時の更新とリトライは `auth.md` に従う。

## 終了コード

`internal/exit` が定義する。`cli.Execute` が `exit.Error` をコードに変換する。

| コード | 意味 |
|---|---|
| 0 | 成功 |
| 1 | 実行時エラー (API 側エラー含む) |
| 2 | 使い方の誤り |
| 3 | 未認証 (`traq auth login` が必要) |
| 4 | スコープ不足 |

## 表示方針

- `view` の関数は `WriteXxx(w io.Writer, ...)` の形に統一する
- `view` は整形だけを行い、チャンネルパス解決は `channel`、メッセージ埋め込み解決は `markdown` が担当する

## テスト方針

- ロジックのテストはドメイン・`view`・`markdown` の単体テストに置き、`cli` のテストは配線・終了コード・実サーバーとの往復に限定する
- 実行手順は `development.md` に従う
