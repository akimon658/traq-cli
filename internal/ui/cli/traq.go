package cli

import (
	"github.com/spf13/cobra"

	"github.com/akimon658/traq-cli/internal/foundation/config"
	"github.com/akimon658/traq-cli/internal/traq"
)

// resolveClient は Env から traQ サービスを作り、トークンを解決して
// 認証済みクライアントを返す。ドメインサービスはこの1個だけ受け取る。
func resolveClient(cmd *cobra.Command, e *config.Env) (*traq.Client, error) {
	svc := traq.NewService(e.Origin, e.ClientID, e.LoadStore(), e.LoadSessions())
	rt, err := svc.ResolveToken(cmd.Context(), e.TokenOverride, e.AccessToken)
	if err != nil {
		return nil, err
	}
	return svc.WithToken(&rt), nil
}
