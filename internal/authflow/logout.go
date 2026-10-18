package authflow

import (
	"context"
	"errors"

	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/config"
	"github.com/akimon658/traq-cli/internal/exit"
)

// Logout は保存済みトークンを失効・削除する。
// OAuth ならサーバー側の revoke を試み、失敗時は警告として返す (削除は続行する)。
// 戻り値 (warning) が nil でなければ呼び出し元が標準エラーに出すこと。
func Logout(ctx context.Context, e *config.Env) (warning, err error) {
	stored, _, found, err := e.LoadStore().Load()
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, exit.AuthRequiredError(
			errors.New("not authenticated: run 'traq auth login' first"),
		)
	}
	origin := e.OriginOf(&stored)
	if stored.Method == auth.MethodOAuth {
		if rerr := auth.Revoke(ctx, origin, stored.AccessToken); rerr != nil {
			warning = rerr
		}
	}
	if _, err := e.LoadStore().Delete(); err != nil {
		return warning, err
	}
	return warning, nil
}
