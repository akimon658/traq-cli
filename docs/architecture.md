# アーキテクチャ

traq-cli のパッケージ構成と設計意図を定める。認証の詳細は `auth.md`、開発手順は `development.md` を参照すること。

## 目的

- 責務ごとにパッケージを分け、どこに何を書くかを一意にする
- 依存方向は一方向であり、循環参照はない
- 純粋な関数と実サーバーが必要な処理はパッケージで分かれており、前者は `-short` で検証できる

## パッケージ構成

```
cmd/traq/              エントリポイント。cli.Execute() を呼ぶだけ
internal/cli/          cobra の配線のみ。auth.go / channel.go / message.go / message_search.go / user.go / root.go
internal/config/       実行設定 Env と既定値解決 (TRAQ_* 環境変数)
internal/session/      トークン解決・401時の refresh リトライ・スコープ事前検査
internal/auth/         OAuth プリミティブ (PKCE / 認可URL / トークン交換 / Token / Store / SessionStore)
internal/authflow/     認証の手順 (login / manual / complete / logout / status 検証)
internal/client/       go-traq の薄ラッパ。origin とトークン注入を隠す
internal/channel/      チャンネル解決・未読取得・分類 (DM→メンション→通知オン→お気に入り→その他)
internal/message/      メッセージ取得・検索・投稿と付随情報の拡充
internal/user/         自ユーザー取得
internal/view/         人間可読テキストの出力専任。API を呼ばない
internal/markdown/     埋め込み解決の純粋パーサ。副作用なし
internal/exit/         終了コード (1/2/3/4) と ExitCodeError
```

依存方向は次の通り。下位は上位を知らない。

```
cli → {authflow, channel, message, user} → session → config → auth
cli → view → {channel, message, auth, markdown}
message → channel, markdown
session → client, auth, exit
```

規約:

- `cli` はフラグ検証 (usage 系) → `session.ResolveToken` → ドメインサービス → `view` の順に呼び出す配線であり、API 呼び出しや整形ロジックは持たない
- ドメインサービス (`channel` / `message` / `user` / `authflow`) はデータを返し、表示は `view` に任せる。`view` を import しない
- `view` は `io.Writer` への出力だけを行い、API を呼ばない。入力型は `message` / `channel` / `auth` の型を参照する
- `session.Exec` が唯一の API 実行経路。401 時の refresh・保存・リトライはここに集約されており、呼び出し元は通常の呼び出しと同じ形で使う
- 終了コードと `exit.Error` は `exit` が定義する。`session` やドメインサービスはこれを返し、`cli.Execute` がコードに変換する

## 実行フロー

1. `config.NewEnv()` で `TRAQ_ORIGIN` / `TRAQ_CLIENT_ID` / `TRAQ_ACCESS_TOKEN` を読み、既定値を補う
2. `globalFlags.apply` で明示指定された `--origin` / `--client-id` / `--token` だけを上書きする
3. `session.ResolveToken` で `--token` → `TRAQ_ACCESS_TOKEN` → 保存済みトークンの順に解決する。期限切れなら refresh して保存する
4. `session.RequireScope` で OAuth トークンのスコープを事前検査する。Bot トークンや `--token` 持ち込みは検査せずサーバー判定に委ねる
5. ドメインサービスが `session.Exec` 経由で API を呼ぶ。401 なら refresh して1回だけリトライする
6. `view` が結果をテキスト整形して出力する

## 終了コード

`internal/exit` が定義する。`cli.Execute` が `exit.Error` をコードに変換する。

| コード | 意味 |
|---|---|
| 0 | 成功 |
| 1 | 実行時エラー (API 側エラー含む) |
| 2 | 使い方の誤り |
| 3 | 未認証 (`traq auth login` が必要) |
| 4 | スコープ不足 |

## 設定・認証運用

- トークン優先順位: `--token` (source `flag`) → `TRAQ_ACCESS_TOKEN` (`environment`) → keyring → credentials ファイル
- `traq auth status` は解決元 (`source`) を表示する。環境変数が保存済みトークンを隠している場合も、この表示で判別できる
- 詳細なフロー・保存先・セキュリティ要件は `auth.md` に従う

## 表示方針

- `view` の関数は `WriteXxx(w io.Writer, ...)` の形に統一する
- メッセージ表示は取得順に関わらず時系列 (古い順) に並べる。未読境界・引用・添付・スタンプの節構成は `view/message.go` が担当する
- チャンネルパス解決は `channel` が担当する (`BuildPathMap` / `FindByPath` / `DisplayLabel`)
- メッセージ・ファイル埋め込み (`!{...}` と末尾 URL) の解決は `markdown` が担当する。コードフェンス・インラインコード内は無視する
- ユーザー名解決に失敗したら `@name` と衝突しない `(ユーザー解決失敗 id:...)` 形式で続行する

## テスト方針

- `-short` で回る単体テスト: `markdown` (パーサ表駆動) / `view` (整形) / `channel` (分類・パス) / `message` (ラベル・ID解決・フラグ検証) / `session` (解決優先順位) / `authflow` (コールバック解析)
- 統合テスト (`internal/cli`, testcontainers + compose.yaml): コマンド配線・終了コード・実サーバーとの往復を検証する。ES 遅延はポーリングで吸収する
- ロジックのテストはドメイン・`view`・`markdown` の単体テストに置き、`cli` のテストは配線・終了コード・実サーバーとの往復に限定する
