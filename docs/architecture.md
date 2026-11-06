# アーキテクチャ

traq-cli のパッケージ構成と設計意図を定める。責務ごとにパッケージを分け、どこに何を書くかを一意にする。

## パッケージ構成

```
cmd/traq/                    エントリポイント。ui/cli.Execute() を呼ぶだけ
internal/foundation/config/  実行設定 Env と既定値解決 (TRAQ_* 環境変数)
internal/foundation/exit/    終了コードと ExitCodeError
internal/foundation/text/    埋め込み解決の純粋パーサ。副作用なし
internal/domain/auth/        認証の関心の Port。Token / Store / SessionStore インターフェース、
                             ManualSession / PKCE / スコープ / コールバック解析などの純粋物。
                             go-traq も infrastructure も import しない
internal/domain/traq/        traQ API の関心の Port。認証済みクライアント Client /
                             MessageQuery / SearchQuery / 表示ラベルなどの純粋物。
                             domain/auth も infrastructure も import しない
internal/domain/channel/     未読取得・分類
internal/domain/file/        ファイル取得 (ダウンロード) と保存
internal/domain/message/     メッセージ取得・検索・投稿
internal/infrastructure/traq/        traQ との通信アダプタ。Service (常駐依存 + 対象別 LRU) /
                             ClientImpl (解決済みトークン束縛・401時 refresh リトライ・
                             スコープ事前検査) / go-traq 呼び出し・トークン解決・
                             認証の手順 (login / manual / complete / logout / status 検証) /
                             ループバックコールバックサーバー / チャンネル解決・表示ラベル。
                             99designs/keyring は触らない
internal/infrastructure/credential/  Token 保存アダプタ。keyring (service 名は `traq-cli`) を
                             優先し、対応外環境では credentials ファイルにフォールバックする
internal/infrastructure/session/     手動ログインセッション保存アダプタ (ファイル)
internal/ui/cli/             cobra の配線のみ。infrastructure の具象生成はここに集約する
internal/ui/view/            人間可読テキストの出力。MessageRenderer が domain/traq.Client を保持し遅延解決する
```

依存方向は次の通り。下位は上位を知らない。

```
ui/cli → domain/{auth,traq} (Port)
ui/cli → infrastructure/{traq,credential,session} (配線・具象生成のみ)
ui/cli → ui/view → {domain, foundation/text}
domain/message, domain/channel, domain/file → domain/traq のみ
ui/view/message → domain/traq、ui/view/auth → domain/auth
infrastructure/traq → domain/{auth,traq} + foundation
infrastructure/{credential,session} → domain/auth のみ
foundation/config → domain/auth のみ (infrastructure を知らない)
```

`domain/auth`・`domain/traq` は `infrastructure` も `go-traq` のクライアント実装も import しない
(`domain/traq` が `go-traq` の DTO 型を参照するのは許容する)。
`infrastructure/traq` が `domain` の汎用部品 (PKCE / SessionStore 等) を使う。

レイヤー:

- `foundation`: 横断的基盤。`exit` / `text` は無依存、`config` は `domain/auth` の Port 型のみ保持する
- `domain/auth` は認証・資格情報の Port のみを持ち、traQ 通信の知識を持たない。`config` を import しない
- `domain/traq` は traQ API の Port のみを持ち、保存や HTTP の知識を持たない
- `infrastructure/traq` は traQ との通信関心を集約する。`Service` (常駐依存 + 対象別 LRU キャッシュ) と `ClientImpl` (解決済みトークンに束縛された認証済みクライアント) を持ち、
  トークン解決・スコープ検査・401 時の refresh リトライ・エラー整形を内包する。呼び出し側は `Client` のメソッドを呼ぶだけでよい。
  同一プロセス内の重複 API 呼び出しは `Client` 内のキャッシュが吸収するため、呼び出し側で取得結果を引き回さない
- `domain`: traQ ドメイン操作。`domain/auth` + `domain/traq` + `foundation` のみに依存
- `ui`: プレゼンテーション。`cli` (配線) と `view` (表示) を束ねる

規約:

- `ui/cli` は配線のみを担う。呼び出し順は フラグ検証 (usage 系) → `resolveClient` (トークン解決+認証済みクライアント化) → ドメインサービス → `ui/view` とし、API 呼び出しや整形ロジックは持たない。`infrastructure` の具象 (`NewService` / `NewDefaultStore` / `NewDefaultSessionStore`) を作ってよいのは `ui/cli` だけである
- ドメインサービス (`domain/channel` / `domain/message` / `domain/file`) はデータを返す。表示は `view` に任せ、`view` を import しない
- ドメインサービスは `config.Env` を保持しない。必要な値 (`domain/traq.Client`) はコンストラクタの明示引数で受け取る。`origin` は重ねて保持せず、`Client.Origin()` を正典とする
- `infrastructure/traq.Service` はコマンド毎に `ui/cli` で1個作り (`resolveClient` 内)、`WithToken` で束縛した `domain/traq.Client` をドメインサービス間で共有する
- `ui/view` は `io.Writer` への出力と表示用の遅延解決を行う。`MessageRenderer` が `domain/traq.Client` を保持し、ユーザー・スタンプ・引用・添付・自ユーザー解決は描画時に行い、失敗時はIDフォールバック表示に倒す。入力型は `domain` の型を参照する
- `domain/traq.Client` が唯一の API 実行経路。401 時の refresh・保存・リトライは `infrastructure/traq.ClientImpl` 内に集約されており、呼び出し元は通常のメソッド呼び出しと同じ形で書ける。一覧系と ID 単位の取得は対象別 LRU に保持される
- 終了コードと `exit.Error` は `foundation/exit` が定義する。`infrastructure/traq` やドメインサービスはこれを返し、`ui/cli.Execute` がコードに変換する

## ドメインサービスの責務分界

- `channel`: 未読取得・分類。`Origin` を必要としない。チャンネル解決・ラベル解決は `infrastructure/traq` に寄せる
- `file`: ファイル取得と保存先解決。`Origin` を必要としない
- `message`: メッセージ取得・検索・投稿。`Origin` は保持せず `Client.Origin()` から取得する。表示に必要な解決は `view.MessageRenderer` が `domain/traq.Client` 経由で遅延取得する
- ユーザー・スタンプ・自ユーザー・チャンネルの単品解決 (`GetUserByID` / `GetStampName` / `GetSelfIDAndGroups` / `ResolveChannelID` / `ResolveUserIDs` / `GetChannelLabel` 等) は `domain/traq.Client` が行う。キャッシュがあるためドメイン側・表示間で結果を引き回さない。一覧系 (`GetUserNameMap` / `GetStampNameMap`) はチャンネルラベル解決など必要な箇所でのみ使う
- `domain/traq` は表示用の純粋なラベル整形を提供し、`domain/channel`・`ui/view`・`infrastructure/traq` が共有する

## 実行フロー

```
foundation/config (Env 解決) → infrastructure/traq (トークン解決・認証済みクライアント化) → ドメインサービス (Client メソッド経由) → ui/view
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

- `ui/view` の出力は `MessageRenderer` メソッドの `WriteXxx(ctx, w, ...)` の形に統一する
- `ui/view` は `domain/traq.Client` の遅延解決と整形を行う。解決失敗はIDフォールバック表示に倒す。チャンネルパス解決は `infrastructure/traq` が、メッセージ埋め込み解決は `foundation/text` が担当する
