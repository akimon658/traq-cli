# traq-cli

[traQ](https://github.com/traPtitech/traQ) を操作するための CLI ツールです。

## 目的

traQ の UI 上で人間が行える操作と同等のことを、AI エージェントが実行できるようにすることを目的としています。

人間がマウスやキーボードで行っている操作を、AI エージェントが外部ツールとして呼び出せる形で提供します。人間向けの便利な UI を提供することは目標としていません。

## 設計思想

- リッチな TUI や対話的な操作は提供しません。
- 非対話 (ワンショット) での実行を基本とし、スクリプトや AI エージェントからの呼び出しで扱いやすい設計を目指します。

## 機能範囲

traQ で UI からできることと同等の操作を、段階的に提供していく予定です。

## ローカルで traQ を動かす

Docker Compose で traQ とそのデータベース (MariaDB) をローカルに立てられます。

```sh
docker compose up -d traq_server
# traQ サーバーが http://localhost:3000 で起動する
# (ポートが使えない場合は TRAQ_SERVER_PORT=3001 docker compose up -d traq_server のように上書きできます)
```

traQ の初期データとして `traq` / `traq` という管理者ユーザーと `#general` チャンネルが自動的に作成されます。

CLI から使う例:

```sh
go build -o traq .
./traq --origin http://localhost:3000 users me --token <アクセストークン>
```

権限・データは `compose.yaml` の volume (`mariadb_data`, `traq_storage`) に永続化されます。消したい場合は:

```sh
docker compose down --volumes
```

## テスト

```sh
go test ./...
```

`internal/cli` のテストは Testcontainers で実 traQ サーバー (compose.yaml) を自動的に起動し、
CLI コマンドの実行結果を検証します。Docker が必要で、起動待ちなどにより完了まで1〜2分かかります。

- Docker が無い環境や素早く回したい場合は `go test -short ./...` で統合テストをスキップできます。

## ドキュメント

- [認証設計](./docs/auth.md)

## ステータス

開発初期です。仕様は予告なく変更される可能性があります。

## ライセンス

[0BSD](./LICENSE)
