# アーキテクチャ

traq-cli のパッケージ構成と設計意図を定める。責務ごとにパッケージを分け、どこに何を書くかを一意にする。

## パッケージ構成

```
cmd/traq/                  エントリポイント。ui/cli.Execute() を呼ぶだけ
internal/foundation/config/  実行設定 Env と既定値解決 (TRAQ_* 環境変数)
internal/foundation/exit/   終了コードと ExitCodeError
internal/foundation/client/ go-traq の薄ラッパ。origin とトークン注入を隠す
internal/foundation/text/   埋め込み解決の純粋パーサ。副作用なし
internal/auth/             認証全般。OAuth プリミティブ (PKCE / 認可URL / トークン交換 / Token / Store / SessionStore)、
                           トークン解決・401時の refresh リトライ・スコープ事前検査、
                           認証の手順 (login / manual / complete / logout / status 検証)
internal/domain/channel/   チャンネル解決・未読取得・分類
internal/domain/file/      ファイル取得 (ダウンロード) と保存
internal/domain/message/   メッセージ取得・検索・投稿と付随情報の拡充
internal/domain/user/      自ユーザー取得 + 解決失敗表示の正規形式
internal/ui/cli/           cobra の配線のみ
internal/ui/view/          人間可読テキストの出力専任。API を呼ばない
```

依存方向は次の通り。下位は上位を知らない。

```
ui/cli → domain → auth → foundation
ui/cli → ui/view → {domain, auth, foundation/text}
domain/message → domain/channel, domain/user, foundation/text
domain/channel → domain/user
auth → foundation/{client, exit}
foundation/config → auth (Store/Token 型の保持)
```

レイヤー:

- `foundation`: 横断的基盤。`exit` / `client` / `text` は無依存、`config` は `auth` の型を保持する
- `auth`: 認証関心の集約。`foundation` のみに依存し、`config` を import しない
  (ResolveToken/Exec 等は `origin/clientID/store` を明示引数で受け取る)
- `domain`: traQ ドメイン操作。`auth` + `foundation` のみに依存
- `ui`: プレゼンテーション。`cli` (配線) と `view` (表示) を束ねる

規約:

- `ui/cli` はフラグ検証 (usage 系) → `auth.ResolveToken` → ドメインサービス → `ui/view` の順に呼び出す配線であり、API 呼び出しや整形ロジックは持たない
- ドメインサービス (`domain/channel` / `domain/message` / `domain/user` / `domain/file`) はデータを返し、表示は `view` に任せる。`view` を import しない
- `ui/view` は `io.Writer` への出力だけを行い、API を呼ばない。入力型は `domain` / `auth` の型を参照する
- `auth.Exec` が唯一の API 実行経路。401 時の refresh・保存・リトライはここに集約されており、呼び出し元は通常の呼び出しと同じ形で使う
- 終了コードと `exit.Error` は `foundation/exit` が定義する。`auth` やドメインサービスはこれを返し、`ui/cli.Execute` がコードに変換する

## 実行フロー

```
foundation/config → auth → ドメインサービス (auth.Exec 経由) → ui/view
```

トークンの解決順・スコープ検査・401 時の更新とリトライは `auth.md` に従う。

## 終了コード

`internal/foundation/exit` が定義する。`ui/cli.Execute` が `exit.Error` をコードに変換する。

| コード | 意味 |
|---|---|
| 0 | 成功 |
| 1 | 実行時エラー (API 側エラー含む) |
| 2 | 使い方の誤り |
| 3 | 未認証 (`traq auth login` が必要) |
| 4 | スコープ不足 |

## 表示方針

- `ui/view` の関数は `WriteXxx(w io.Writer, ...)` の形に統一する
- `ui/view` は整形だけを行い、チャンネルパス解決は `domain/channel`、メッセージ埋め込み解決は `foundation/text` が担当する

## テスト方針

- ロジックのテストはドメイン・`ui/view`・`foundation/text` の単体テストに置き、`ui/cli` のテストは配線・終了コード・実サーバーとの往復に限定する
- 実行手順は `development.md` に従う
