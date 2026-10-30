# 認証設計

traq-cli の認証設計を定める。traQ サーバー (traPtitech/traQ) の実装を検証した上で決めた事項を含む。

## 認証モード

3 つの認証モードを提供する。すべての API 呼び出しは `Authorization: Bearer <token>` で統一される。

| モード | 用途 | 取得方法 | 有効期限 | refresh | スコープ事前チェック |
|---|---|---|---|---|---|
| Bot トークン | Bot 独自の権限 (ユーザーと独立、チャンネル購読で制限) | Bot 管理画面で発行 → `traq auth login --token` | 無期限 | なし | 不可 (サーバーの 403 をそのまま整形) |
| OAuth 読取専用 | ユーザーのプライベートリソースも読める | `traq auth login --browser --scope read` | あり (デフォルト 365 日) | あり (サーバー設定次第) | あり |
| OAuth 読み書き | ユーザーと同等の権限 | `traq auth login --browser --scope read,write` | あり (同上) | あり (同上) | あり |

備考:

- traQ のスコープはフラットであり、`write` に `read` は含まれない。読み書きモードでは `read write` を両方要求する。
- 読み取り専用をデフォルトとし、必要に応じて書き込み権限を取得させる (最小権限の原則)。

## 検証済み traQ 仕様

traPtitech/traQ (master) のコード確認に基づく前提。

- OAuth2 フローは authorization code のみ。device flow (RFC 8628) と `urn:ietf:wg:oauth:2.0:oob` は非対応
- PKCE (`S256`/`plain`) 対応。公開クライアント (confidential でない) は client_id + PKCE のみで動作し、client_secret は不要
- スコープは `read`, `write`, `manage_bot`, `openid`, `profile`
- redirect URI は RFC 8252 のループバック規則を実装済み: 実行時にポートが異なってもよいが、ホストは必ずループバック IP (`127.0.0.1` / `::1`) であること。`localhost` は `net.ParseIP` に通らないため不可。パスは登録と完全一致が必要
- アクセストークン有効期限のデフォルトは 365 日 (`oauth2.accessTokenExp`)。リフレッシュトークン発行はサーバー設定 (`oauth2.isRefreshEnabled`) 次第で、デフォルトは無効 (機能自体は実装あり)
- 認可コードの寿命は 5 分・1 回限り
- password grant (`grant_type=password`) は traQ に実装があるが、本 CLI では採用しない
- `GET /users/me` のレスポンス `MyUserDetail` には `bot` (bool) と `permissions` が含まれる。これをトークンの検証・方法判別・権限表示に使う

## CLI サーフェス

```
traq auth login --browser [--scope read|read,write]      # OAuth code flow (PKCE + ループバック)
traq auth login --manual [--scope read|read,write]       # ブラウザを開かず認可 URL のみ出力 (auth complete と組み合わせる)
traq auth complete <URL>                                 # 手動ログインを完了する
traq auth login --token <TOKEN>                          # Bot トークン / 手動取得の OAuth トークン (自動判別)
traq auth logout
traq auth status
```

対話的な選択 UI は提供しない。実行形態ごとにフラグを明示する。

### OAuth モードを動かすための前提作業

traQ サーバー上 (q.trap.jp) に traq-cli 用の OAuth2 クライアントを一度だけ登録する。

- redirect URI: `http://127.0.0.1/callback` (ポートは登録不要。実行時は任意ポートでリッスンする)
- confidential: オフ
- スコープ: `read`, `write` (将来用に `manage_bot` も申請してよい)

発行された client_id は実装では `internal/auth` に定数として埋め込み、self-host・検証環境のために `--client-id` フラグ、および `TRAQ_CLIENT_ID` 環境変数で上書きできるようにする。

## ブラウザログインフロー (RFC 8252 authorization code + PKCE)

`traq auth login --browser` の手順:

1. セキュア乱数で code_verifier (64 バイト) と state を生成。challenge は S256(verifier)
2. `127.0.0.1` のエフェメラルポートでコールバックサーバーを起動
3. 認可 URL を組み立てて標準出力に表示し、可能ならブラウザで開く
4. ブラウザで traQ にログインし、要求スコープを承認
5. traQ が `http://127.0.0.1:<port>/callback?code=...&state=...` へリダイレクト
6. コールバックサーバーは code と state を検証 (state 不一致や error パラメータは即エラー) し、ブラウザに「このタブを閉じて構いません」を返す
7. `{origin}/api/v3/oauth2/token` へ form で `grant_type=authorization_code`, `code`, `redirect_uri` (リクエストと同一文字列), `client_id`, `code_verifier` を送り、トークンレスポンス (`access_token`, `refresh_token`, `expires_in`, `scope`) を受け取る
8. `GET /users/me` でトークンを検証し、`bot` フラグ・ユーザー名・権限を取得
9. トークンを保存して完了

全体のタイムアウトは 5 分 (traQ の認可コード寿命に合わせる)。

### 手動ログインフロー (--manual + auth complete)

SSH 先や headless 環境など、ブラウザが同マシンで開けない場合の 2 ステップフロー。traQ が device flow / OOB に非対応のため、リダイレクトを手動で転送する。

1. `traq auth login --manual [--scope ...]`:
   - PKCE (verifier / state / challenge) を生成し、セッションを `login-session.json` に保存する
   - redirect URI はローカルサーバーを立てないため登録時と同じ `http://127.0.0.1/callback` (ポート暗黙 80) を使う
   - 認可 URL と完了コマンド (`traq auth complete "<URL>"`) を出力して終了する (ブラウザも開かない)
2. ユーザーがブラウザ上でtraQにログインして承認すると、リダイレクト先 (CLI 実行マシンのループバック `http://127.0.0.1/callback`) はページとしては開けないが、URL はアドレスバーに残る。その URL 全体をコピーして `complete` を実行する環境へ転送する
3. `traq auth complete <URL>`:
   - 引数は完全 URL のほか `?code=...` / `code=...&state=...` のようなクエリ文字列も受け付ける
   - `error` パラメータ (拒否) は exit 1、形式不正は exit 2、セッション不在は exit 3
   - state (含まれる場合) をセッションと照合し、不一致なら失敗する
   - セッション記録の origin / client_id / redirect_uri / verifier を使ってトークン交換し、ブラウザフローと同じ後半処理 (検証・保存・出力) に合流する

仕様:

- セッションは `UserConfigDir/traq-cli/login-session.json` に 0600 (ディレクトリ 0700) で保存する。一時ファイルであり keyring は使わない
- セッションの寿命 (`auth.ManualSessionTTL`) は 15 分。期限切れセッションは Load 時に削除される
- 認可コードは 1 回限りで寿命 5 分。トークン保存成功時にセッションを削除し、交換に失敗した場合は再試行のため残す
- `login --manual` の再実行でセッションは上書きされる (複数セッションは保持しない)
- `--manual` は `--browser` / `--token` と併用できない (exit 2)

ブラウザが同マシンで開けない場合でも、bot トークンや手動取得したトークンの貼り付け (`traq auth login --token`) はクラシックなフォールバックとして使える。

## クレデンシャルの扱い

### トークンの保存先

keyring (macOS Keychain など, service 名は `traq-cli`) を優先し、対応外環境 (CI・コンテナ) では `~/.config/traq-cli/credentials` (パーミッション 0600, ディレクトリ 0700) にフォールバックする。フォールバック時は標準エラーに警告を出す。

保存する情報:

- `method`: `bot` / `oauth`
- `accessToken`
- `refreshToken` (発行された場合のみ)
- `scope`
- `origin`
- `userID`, `userName`, `userBot` (`/users/me` より)
- `issuedAt`, `expiresAt` (存在する場合)

### クレデンシャルの解決優先順位

実行時のトークンは次の順で解決する。

1. `--token` フラグ (root persistent flag, ワンショット上書き)
2. `TRAQ_ACCESS_TOKEN` 環境変数 (CI・エージェント用。`TRAQ_ORIGIN` も同様)
3. keyring → credentials ファイル

`traq auth status` はどのソースのトークンかを表示する。環境変数の古いトークンが保存済みトークンを隠してしまっている事故を防ぐためである。

### セキュリティ要件

- トークンはログやエラー文言に出さない (常にマスク)
- credentials ファイルは 0600 で作成する
- コールバックは state 検証により、外部から localhost に送りつけられる偽 code を排除する
- OAuth トークンの無効化は `POST /oauth2/revoke` を行い、サーバー側からも失効させる

## 実行時のトークン運用

### スコープ事前チェック

各 API コマンドは要求スコープ (`read` / `write`) を宣言し、保存済み OAuth トークンの `scope` と実行前に照合する。

- 不足している場合、API を呼ばずに機械可読なエラーで終了し、`traq auth login --browser --scope read,write` での再ログインを提案する
- Bot トークンとスコープ不明な手動トークン (`--token` で入れたもの) は事前チェックを行わず、サーバーの判定 (403) をそのまま整形して返す

### 401 時の挙動

- OAuth トークンで refresh_token を保持する場合: 1 回だけ `grant_type=refresh_token` で更新し、リトライする。更新に失敗 (`invalid_grant` など) したら保存を削除し `traq auth login` を促す
- Bot トークン: refresh できないため、すぐ再設定を促すメッセージを出す

### logout / status

- `traq auth logout`: OAuth なら `/oauth2/revoke` を実行してからローカルを削除。Bot はローカル削除のみ
- `traq auth status`: method, ユーザー (`@name`, bot かどうか), スコープ, 有効期限, トークンの取得元を表示

## パッケージ構成

パッケージ全体の構成と依存方向は [`architecture.md`](./architecture.md) を参照すること。
認証に関わる主要部品は `internal/auth/` (プリミティブ) と `internal/authflow/` (手順) に置く。
主要依存: `spf13/cobra`, `traPtitech/go-traq`, `99designs/keyring`。PKCE とトークン交換は標準ライブラリで自前実装する。

## エラーと終了コード

AI エージェントからの利用を想定し、終了コードと 1 行の機械可読な標準エラーで挙動を示す。終了コードの定義は [`architecture.md`](./architecture.md) に従う。

## 採用しないこと

- password grant (`--username/--password` による非対話ログイン): traQ に実装はあるが OAuth 2.1 で非推奨のため見合わせる
- device flow / OOB: traQ が非対応のため不可能。代替は手動トークン貼り付け
- 複数プロファイル: 保存できるトークンは 1 つ。用途の切り替えは `--token` や `TRAQ_ACCESS_TOKEN` で行う
