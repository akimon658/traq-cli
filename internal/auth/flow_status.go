package auth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/foundation/client"
	"github.com/akimon658/traq-cli/internal/foundation/exit"
)

// VerifyToken はアクセストークンを /users/me で検証し、ユーザー詳細を返す。
// 検証失敗時は整形済みエラーを返す。呼び出し元は取得済みの表示 (method/user/scope等) を
// 先に出し、このエラーを exit 1 として扱うこと。
func VerifyToken(
	ctx context.Context,
	c *client.Client,
	accessToken string,
) (*traq.MyUserDetail, *http.Response, error) {
	me, resp, err := c.GetMe(ctx, accessToken)
	if err != nil {
		meErr := client.WrapError(err, resp)
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			meErr = fmt.Errorf("the token was rejected: %v; run 'traq auth login'", meErr)
		}
		return nil, resp, exit.GeneralError(fmt.Errorf("failed to verify the token: %v", meErr))
	}
	return me, resp, nil
}
