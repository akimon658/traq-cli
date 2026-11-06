package cli

import (
	"github.com/spf13/cobra"

	"github.com/akimon658/traq-cli/internal/domain/auth"
	"github.com/akimon658/traq-cli/internal/domain/traq"
	"github.com/akimon658/traq-cli/internal/foundation/config"
	infracredential "github.com/akimon658/traq-cli/internal/infrastructure/credential"
	infrasession "github.com/akimon658/traq-cli/internal/infrastructure/session"
	infratraq "github.com/akimon658/traq-cli/internal/infrastructure/traq"
)

// resolveFunc は認証済みクライアントの解決子。各コマンド実行時に呼ばれ、
// テストは newRootCmd にスタブを注入できる (パッケージ変数を持たないため t.Parallel 可)。
type resolveFunc func(cmd *cobra.Command, e *config.Env) (traq.Client, error)

// loadStore は保存先を返す。Env.Store が nil の場合は既定の Store を使う。
func loadStore(e *config.Env) auth.Store {
	if e.Store == nil {
		return infracredential.NewDefaultStore()
	}
	return e.Store
}

// loadSessions は手動ログインのセッション保存先を返す。nil の場合は既定を使う。
func loadSessions(e *config.Env) auth.SessionStore {
	if e.Sessions == nil {
		return infrasession.NewDefaultSessionStore()
	}
	return e.Sessions
}

// newService は Env から traQ サービスを作る。配線は cli に集約し、
// foundation/config が infrastructure を知らないようにする。
func newService(e *config.Env) *infratraq.Service {
	return infratraq.NewService(e.Origin, e.ClientID, loadStore(e), loadSessions(e))
}

// defaultResolveClient は Env から traQ サービスを作り、トークンを解決して
// 認証済みクライアントを返す。ドメインサービスはこの1個だけ受け取る。
func defaultResolveClient(cmd *cobra.Command, e *config.Env) (traq.Client, error) {
	svc := newService(e)
	rt, err := svc.ResolveToken(cmd.Context(), e.TokenOverride, e.AccessToken)
	if err != nil {
		return nil, err
	}
	return svc.WithToken(&rt), nil
}
