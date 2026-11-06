# 開発者向けガイド

## ローカルで traQ を動かす

Docker Compose で traQ とそのデータベース (MariaDB) をローカルに立てられる。

```sh
docker compose up -d traq_server
# traQ サーバーが http://localhost:3000 で起動する
# (ポートが使えない場合は TRAQ_SERVER_PORT=3001 docker compose up -d traq_server のように上書きできる)
```

traQ の初期データとして `traq` / `traq` という管理者ユーザーと `#general` チャンネルが自動的に作成される。

CLI から使う例:

```sh
go run ./cmd/traq --origin http://localhost:3000 users me --token <アクセストークン>
```

権限・データは `compose.yaml` の volume (`mariadb_data`, `traq_storage`) に永続化される。消したい場合は:

```sh
docker compose down --volumes
```

## テスト

```sh
go test ./...
```

`internal/infrastructure/traq` のテストは Testcontainers で compose.yaml の実 traQ サーバーを起動し、
`Client`・`Service` の実サーバーとの往復を検証する。Docker が必要で、起動待ちなどにより完了まで1〜2分かかる。
メッセージ検索の検証では ES の反映遅延をポーリングで吸収する。

- Docker が無い環境や素早く回したい場合は `go test -short ./...` で実サーバーを使うテストをスキップできる。`-short` で回るのは実サーバー不要の単体テストである。
  `foundation/text` / `ui/view` / `domain/channel` / `domain/message` / `domain/file` / `domain/auth` / `domain/traq` / `infrastructure/credential` / `infrastructure/session` / `ui/cli`。実サーバーは不要である。
- `ui/cli` で実サーバーは叩かない。配線の確認はモックで行う。

## モック生成

インターフェースを変更したときは、リポジトリルートで以下を実行してモックを再生成する。

```sh
go generate ./...
```

`//go:generate` の `-destination` はパッケージディレクトリからの相対パスで書かれている。
リポジトリルートから `go tool mockgen -destination=internal/domain/traq/mock_client.go ...` のように手動実行して destination を上書きしないこと。
生成ヘッダーがずれて、次回の `go generate` で余計な差分が出る。
