# テスト構成

## 方針

- 個別機能は各層の単体テストで検証する
- 対象外の層はモックする
- 実 traQ が必要な確認は `internal/traq` に寄せる。`ui/cli` で実サーバーは叩かない

## 層とモック境界

- `foundation`・`view`・`Validate`・`Parse` 系: 純粋な引数→戻り値の検証。モックなし
- `domain`: `traq.Client` (実装は `ClientImpl`) を mockgen 生成の `MockClient` で差し替える。API 越し・表示は見ない
- `traq`: 実 traQ (Testcontainers) への薄い往復で検証する。`Store` は `MockStore` を使う。汎用 OAuth 部品は `auth` の単体に委ねる
- `auth`: PKCE・`CallbackServer`・`ParseCallback`・`SessionStore` の単体。traQ 固有知識は持たない
- `ui/cli`: 配線のみ。`traq.MockClient` を `resolve` 注入で差し替え、終了コードのみ検証する。出力内容は検証しない (内容は `view` 側で検証)。HTTP・実サーバーは使わない

## 配置・命名規約

- 対象と同パッケージの `*_test.go` に置く
- テスト関数は対象メソッド・関数ごとに1つ作り、ケースは `t.Run` サブテストに書く
  - 例: `TestService_List`、`TestClient_ResolveChannelID`、`TestWriteMessageList`、`TestValidateFileID`
  - 同一対象を `TestMessageList` / `TestMessageListLimitSummary` のように分割しない
- 実 traQ を使うテストは `internal/traq` の `traq_test.go` の `TestMain` が起動する compose スタックを使い、`-short` ではスキップする
