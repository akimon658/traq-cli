package user

import (
	"context"
	"net/http"

	"github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/client"
	"github.com/akimon658/traq-cli/internal/config"
	"github.com/akimon658/traq-cli/internal/session"
)

// FetchMe は自分のユーザー詳細を取得する。
func FetchMe(
	ctx context.Context,
	e *config.Env,
	rt *session.ResolvedToken,
	c *client.Client,
) (*traq.MyUserDetail, error) {
	if err := session.RequireScope(rt, auth.ScopeRead); err != nil {
		return nil, err
	}
	var me *traq.MyUserDetail
	if err := session.Exec(ctx, e, rt, func(ctx context.Context) (*http.Response, error) {
		var resp *http.Response
		var apiErr error
		me, resp, apiErr = c.GetMe(ctx, rt.Token.AccessToken)
		return resp, apiErr
	}); err != nil {
		return nil, err
	}
	return me, nil
}
