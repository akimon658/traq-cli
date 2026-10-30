package auth

import (
	"context"
	"errors"

	"github.com/akimon658/traq-cli/internal/foundation/exit"
)

// Logout は保存済みトークンを失効・削除する。
// OAuth ならサーバー側の revoke を試み、失敗時は警告として返す (削除は続行する)。
// 戻り値 (warning) が nil でなければ呼び出し元が標準エラーに出すこと。
func Logout(ctx context.Context, origin string, store Store) (warning, err error) {
	stored, _, found, err := store.Load()
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, exit.AuthRequiredError(
			errors.New("not authenticated: run 'traq auth login' first"),
		)
	}
	if stored.Method == MethodOAuth {
		if rerr := Revoke(ctx, stored.OriginOr(origin), stored.AccessToken); rerr != nil {
			warning = rerr
		}
	}
	if _, err := store.Delete(); err != nil {
		return warning, err
	}
	return warning, nil
}
