package traq

import (
	"context"
	"fmt"
	"net/http"

	"github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/foundation/exit"
)

// Status は現在の認証状態の照会結果を保持する。
// VerifyErr が nil でなければトークン検証に失敗しているが、
// 取得済みの表示 (method/user/scope等) は Token/Source に入っているため
// 呼び出し元は表示を先に出し、VerifyErr を exit 1 として扱うこと。
// 戻り値 error はトークン解決の失敗 (未認証・期限切れ等) であり、
// この場合は表示を行わずにそのまま返すこと。
type Status struct {
	Origin    string
	Source    string
	Token     Token
	Me        *traq.MyUserDetail
	VerifyErr error
}

// Status はトークンを解決し、/users/me で検証して状態を返す。
// 検証成功時は Token にユーザー情報を補完する。
func (s *Service) Status(
	ctx context.Context,
	tokenOverride, accessToken string,
) (Status, error) {
	rt, err := s.ResolveToken(ctx, tokenOverride, accessToken)
	if err != nil {
		return Status{}, err
	}
	me, _, meErr := s.VerifyToken(ctx, rt.Token.AccessToken)
	if me != nil {
		rt.Token.UserID = me.Id
		rt.Token.UserName = me.Name
		rt.Token.UserBot = me.Bot
	}
	return Status{
		Origin:    s.origin,
		Source:    rt.Source,
		Token:     *rt.Token,
		Me:        me,
		VerifyErr: meErr,
	}, nil
}

// VerifyToken はアクセストークンを /users/me で検証し、ユーザー詳細を返す。
// 検証失敗時は整形済みエラーを返す。呼び出し元は取得済みの表示 (method/user/scope等) を
// 先に出し、このエラーを exit 1 として扱うこと。
func (s *Service) VerifyToken(
	ctx context.Context,
	accessToken string,
) (*traq.MyUserDetail, *http.Response, error) {
	me, resp, err := s.api.MeAPI.GetMe(TokenContext(ctx, accessToken)).Execute()
	if err != nil {
		meErr := WrapError(err, resp)
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			meErr = fmt.Errorf("the token was rejected: %v; run 'traq auth login'", meErr)
		}
		return nil, resp, exit.GeneralError(fmt.Errorf("failed to verify the token: %v", meErr))
	}
	return me, resp, nil
}
