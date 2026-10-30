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

// FetchMe は自分のユーザー詳細を取得する。
func FetchMe(
	ctx context.Context,
	e *config.Env,
	rt *auth.ResolvedToken,
	c *client.Client,
) (*traq.MyUserDetail, error) {
	if err := auth.RequireScope(rt, auth.ScopeRead); err != nil {
		return nil, err
	}
	var me *traq.MyUserDetail
	if err := auth.Exec(
		ctx,
		e.Origin,
		e.ClientID,
		e.LoadStore(),
		rt,
		func(ctx context.Context) (*http.Response, error) {
			var resp *http.Response
			var apiErr error
			me, resp, apiErr = c.GetMe(ctx, rt.Token.AccessToken)
			return resp, apiErr
		},
	); err != nil {
		return nil, err
	}
	return me, nil
}
