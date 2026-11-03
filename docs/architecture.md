# アーキテクチャ

traq-cli のパッケージ構成と設計意図を定める。責務ごとにパッケージを分け、どこに何を書くかを一意にする。

## パッケージ構成

```
cmd/traq/                    エントリポイント。ui/cli.Execute() を呼ぶだけ
internal/foundation/config/  実行設定 Env と既定値解決 (TRAQ_* 環境変数)
internal/foundation/exit/    終了コードと ExitCodeError
internal/foundation/text/    埋め込み解決の純粋パーサ。副作用なし
internal/auth/               汎用 OAuth プリミティブ。PKCE / ループバックコールバックサーバー /
                             手動ログインのセッション保存 (SessionStore) / コールバック解析。
                             traQ 固有の型も go-traq も import しない
internal/traq/               traQ への唯一の出入口。Token / Store / 解決済みトークン / 認証済みクライアント、
                             go-traq 呼び出し・トークン解決・401時の refresh リトライ・スコープ事前検査、
                             認証の手順 (login / manual / complete / logout / status 検証)を集約する
internal/domain/channel/     チャンネル解決・未読取得・分類
internal/domain/file/        ファイル取得 (ダウンロード) と保存
internal/domain/message/     メッセージ取得・検索・投稿と付随情報の拡充
internal/domain/user/        自ユーザー取得 (FetchMe, `user me` 用) とユーザー解決失敗時の共通表示形式 (FallbackLabel)
internal/ui/cli/             cobra の配線のみ
internal/ui/view/            人間可読テキストの出力専任。API を呼ばない
```

依存方向は次の通り。下位は上位を知らない。

```
ui/cli → domain → traq → {auth, foundation}
ui/cli → traq (認証の手順・トークン解決)
ui/cli → ui/view → {domain, traq, foundation/text}
domain/message → domain/channel, foundation/text
domain/message → domain/user (FallbackLabel の参照のみ。Service 依存なし)
domain/channel → domain/user (FallbackLabel の参照のみ。Service 依存なし)
foundation/config → {traq (Store/Token 型・既定値), auth (SessionStore 型)}
```

`auth` は go-traq も `traq` も import しない。`traq` が `auth` の汎用部品 (PKCE / CallbackServer / SessionStore) を使う。

レイヤー:

- `foundation`: 横断的基盤。`exit` / `text` は無依存、`config` は `traq` / `auth` の型を保持する
- `auth` は汎用 OAuth 部品のみを持ち、traQ 固有の知識を持たない。`config` を import しない
- `traq` は traQ との通信関心を集約する。`Service` (常駐依存) と `Client` (解決済みトークンに束縛された認証済みクライアント) を持ち、
  トークン解決・スコープ検査・401 時の refresh リトライ・エラー整形を内包する。呼び出し側は `Client` のメソッドを呼ぶだけでよい
- `domain`: traQ ドメイン操作。`traq` + `foundation` のみに依存
- `ui`: プレゼンテーション。`cli` (配線) と `view` (表示) を束ねる

規約:

- `ui/cli` は配線のみを担う。呼び出し順は フラグ検証 (usage 系) → `resolveClient` (トークン解決+認証済みクライアント化) → ドメインサービス → `ui/view` とし、API 呼び出しや整形ロジックは持たない
- ドメインサービス (`domain/channel` / `domain/message` / `domain/user` / `domain/file`) はデータを返す。表示は `view` に任せ、`view` を import しない
- ドメインサービスは `config.Env` を保持しない。必要な値 (`*traq.Client`) はコンストラクタの明示引数で受け取る。`origin` は重ねて保持せず、`Client.Origin()` を正典とする
- `traq.Service` はコマンド毎に `ui/cli` で1個作り (`resolveClient` 内)、`WithToken` で束縛した `*traq.Client` をドメインサービス間で共有する。`message` が内包する `channel` も同一 `*traq.Client` で作る
- `ui/view` は `io.Writer` への出力だけを行い、API を呼ばない。入力型は `domain` / `traq` の型を参照する
- `traq.Client` が唯一の API 実行経路。401 時の refresh・保存・リトライは `Client` 内に集約されており、呼び出し元は通常のメソッド呼び出しと同じ形で書ける
- 終了コードと `exit.Error` は `foundation/exit` が定義する。`traq` やドメインサービスはこれを返し、`ui/cli.Execute` がコードに変換する

## ドメインサービスの責務分界

- `channel`: チャンネル指定の解決・未読取得・分類。`Origin` を必要としない
- `file`: ファイル取得と保存先解決。`Origin` を必要としない
- `user`: `FetchMe` による自ユーザー取得 (`user me` 用)。ユーザー一覧は持たない
- `message`: メッセージ取得・検索・投稿と付随情報の拡充 (`Enrichment`)。`Origin` は保持せず `Client.Origin()` から取得し、埋め込み解決と結果表示に使う。内包する `channel` は共有された同一 `*traq.Client` で作る
- ユーザー一覧の取得は `message.FetchEnrichment` が行う。表示条件に依存するため `message` 側に置き、`user` 側に寄せない
- 自ID・所属グループの取得は `message.fetchSelfBestEffort` が同一 `*traq.Client` で行う。`user.Service` を毎回生成しないための意図的重複であり、`user.FetchMe` と仕様変更時は両方を見ること
- `user.FallbackLabel` は `Service` 外の純粋関数であり、ユーザー解決失敗時の正規の表示形式。`channel` と `view` が共有する

## 実行フロー

```
foundation/config (Env 解決) → traq (トークン解決・認証済みクライアント化) → ドメインサービス (Client メソッド経由) → ui/view
```

トークンの解決順・スコープ検査・401 時の更新とリトライは `auth.md` に従う。

## 終了コード

終了コードは `internal/foundation/exit` が定義する。`ui/cli.Execute` が `exit.Error` をコードに変換する。

| コード | 意味 |
|---|---|
| 0 | 成功 |
| 1 | 実行時エラー (API 側エラー含む) |
| 2 | 使い方の誤り |
| 3 | 未認証 (`traq auth login` が必要) |
| 4 | スコープ不足 |

## 表示方針

- `ui/view` の関数は `WriteXxx(w io.Writer, ...)` の形に統一する
- `ui/view` は整形だけを行う。チャンネルパス解決は `domain/channel` が、メッセージ埋め込み解決は `foundation/text` が担当する

## テスト方針

- ロジックのテストはドメイン・`ui/view`・`foundation/text` の単体テストに置き、`ui/cli` のテストは配線・終了コード・実サーバーとの往復に限定する
- 実行手順は `development.md` に従う
