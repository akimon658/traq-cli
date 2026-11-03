package user

import (
	"context"
	"fmt"
	"net/http"

	"github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/foundation/client"
	"github.com/akimon658/traq-cli/internal/foundation/config"
)

// FallbackLabel はユーザーIDが解決できなかったときの表示を返す。
// channel と view で共有する正規の形式。
func FallbackLabel(id string) string {
	return fmt.Sprintf("(ユーザー解決失敗 id:%s)", id)
}

// Service はユーザー操作に必要な常駐依存を束ねる。
type Service struct {
	env    *config.Env
	client *client.Client
	auth   *auth.Service
}

// NewService はユーザーサービスを作る。
func NewService(env *config.Env, client *client.Client) *Service {
	return &Service{
		env:    env,
		client: client,
		auth: auth.NewService(
			env.Origin,
			env.ClientID,
			env.LoadStore(),
			env.LoadSessions(),
		),
	}
}

// FetchMe は自分のユーザー詳細を取得する。
func (s *Service) FetchMe(
	ctx context.Context,
	rt *auth.ResolvedToken,
) (*traq.MyUserDetail, error) {
	if err := auth.RequireScope(rt, auth.ScopeRead); err != nil {
		return nil, err
	}
	var me *traq.MyUserDetail
	if err := s.auth.Exec(
		ctx,
		rt,
		func(ctx context.Context) (*http.Response, error) {
			var resp *http.Response
			var apiErr error
			me, resp, apiErr = s.client.GetMe(ctx, rt.Token.AccessToken)
			return resp, apiErr
		},
	); err != nil {
		return nil, err
	}
	return me, nil
}
