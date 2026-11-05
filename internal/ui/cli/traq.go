package cli

import (
	"github.com/spf13/cobra"

	"github.com/akimon658/traq-cli/internal/foundation/config"
	"github.com/akimon658/traq-cli/internal/traq"
)

// resolveFunc は認証済みクライアントの解決子。各コマンド実行時に呼ばれ、
// テストは newRootCmd にスタブを注入できる (パッケージ変数を持たないため t.Parallel 可)。
type resolveFunc func(cmd *cobra.Command, e *config.Env) (traq.Client, error)

// defaultResolveClient は Env から traQ サービスを作り、トークンを解決して
// 認証済みクライアントを返す。ドメインサービスはこの1個だけ受け取る。
func defaultResolveClient(cmd *cobra.Command, e *config.Env) (traq.Client, error) {
	svc := traq.NewService(e.Origin, e.ClientID, e.LoadStore(), e.LoadSessions())
	rt, err := svc.ResolveToken(cmd.Context(), e.TokenOverride, e.AccessToken)
	if err != nil {
		return nil, err
	}
	return svc.WithToken(&rt), nil
}
