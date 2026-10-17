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
go build -o traq .
./traq --origin http://localhost:3000 users me --token <アクセストークン>
```

権限・データは `compose.yaml` の volume (`mariadb_data`, `traq_storage`) に永続化される。消したい場合は:

```sh
docker compose down --volumes
```

## テスト

```sh
go test ./...
```

`internal/cli` のテストは Testcontainers で実 traQ サーバー (compose.yaml) を自動的に起動し、
CLI コマンドの実行結果を検証する。Docker が必要で、起動待ちなどにより完了まで1〜2分かかる。

- Docker が無い環境や素早く回したい場合は `go test -short ./...` で統合テストをスキップできる。
