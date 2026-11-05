package traq

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/foundation/exit"
)

// Client は解決済みトークンに束縛された認証済みクライアントである。
// Service.WithToken で作り、トークン注入・スコープ検査・401時リトライ・エラー整形を内包する。
// 一覧系と ID 単位の取得は対象別 LRU で保持し、同一プロセス内の重複呼び出しを抑える。
// 呼び出し側は ctx のみを渡せばよく、*http.Response やトークンを意識しない。
type Client struct {
	svc *Service
	rt  *ResolvedToken
}

// Origin は表示用・埋め込み検出用の origin を返す。正典は Service。
func (c *Client) Origin() string {
	return c.svc.origin
}

// Clone はトークンを値コピーした独立クライアントを返す。
// 並列取得で各 goroutine が別トークンを参照し、リトライ時の書き戻しの
// データ競合を避けるために使う。キャッシュは svc 経由で共有される。
func (c *Client) Clone() *Client {
	tok := *c.rt.Token
	return &Client{svc: c.svc, rt: &ResolvedToken{Token: &tok, Source: c.rt.Source}}
}

// requireScope は保存済み OAuth トークンのスコープを事前検査する。
// Bot トークンやスコープ不明な手動トークン (--token) は検査せず、サーバーの判定に委ねる。
func (c *Client) requireScope(scope string) error {
	if c.rt.Token.LacksScope(scope) {
		required := scope
		if scope == ScopeWrite {
			required = ScopeRead + "," + ScopeWrite
		}
		return exit.ScopeInsufficientError(
			fmt.Errorf(
				"the current token does not have the '%s' scope; re-run 'traq auth login --browser --scope %s'",
				scope,
				required,
			),
		)
	}
	return nil
}

// exec runs fn with token injection and one 401 retry, returning the value.
func exec[T any](
	ctx context.Context,
	c *Client,
	fn func(ctx context.Context) (T, *http.Response, error),
) (T, error) {
	var zero T
	out, resp, err := fn(TokenContext(ctx, c.rt.Token.AccessToken))
	if resp != nil && resp.StatusCode == http.StatusUnauthorized {
		nt, rerr := c.svc.RefreshToken(ctx, c.rt.Token)
		if rerr != nil {
			return zero, exit.AuthRequiredError(
				fmt.Errorf(
					"the token was rejected and cannot be refreshed (%v); run 'traq auth login'",
					rerr,
				),
			)
		}
		if _, serr := c.svc.store.Save(nt); serr != nil {
			return zero, serr
		}
		*c.rt.Token = nt
		out, resp, err = fn(TokenContext(ctx, c.rt.Token.AccessToken))
	}
	if err != nil {
		return zero, exit.GeneralError(WrapError(err, resp))
	}
	return out, nil
}

// GetMe は自分のユーザー詳細を取得する。
func (c *Client) GetMe(ctx context.Context) (*traq.MyUserDetail, error) {
	if err := c.requireScope(ScopeRead); err != nil {
		return nil, err
	}
	if v, ok := c.svc.caches.me.Get(singleCacheKey); ok {
		return v, nil
	}
	out, err := exec(ctx, c, c.svc.rawGetMe)
	if err != nil {
		return nil, err
	}
	c.svc.caches.me.Add(singleCacheKey, out)
	return out, nil
}

// PostMessage はチャンネルにメッセージを投稿する。
func (c *Client) PostMessage(
	ctx context.Context, channelID, content string,
) (*traq.Message, error) {
	if err := c.requireScope(ScopeWrite); err != nil {
		return nil, err
	}
	return exec(ctx, c, func(ctx context.Context) (*traq.Message, *http.Response, error) {
		return c.svc.rawPostMessage(ctx, channelID, content)
	})
}

// GetChannelMessages はチャンネルのメッセージを取得する。
func (c *Client) GetChannelMessages(
	ctx context.Context,
	channelID string,
	q MessageQuery,
) ([]traq.Message, error) {
	if err := c.requireScope(ScopeRead); err != nil {
		return nil, err
	}
	return exec(ctx, c, func(ctx context.Context) ([]traq.Message, *http.Response, error) {
		return c.svc.rawGetChannelMessages(ctx, channelID, q)
	})
}

// SearchMessages はメッセージを検索する。
func (c *Client) SearchMessages(
	ctx context.Context,
	q SearchQuery,
) (*traq.MessageSearchResult, error) {
	if err := c.requireScope(ScopeRead); err != nil {
		return nil, err
	}
	return exec(
		ctx,
		c,
		func(ctx context.Context) (*traq.MessageSearchResult, *http.Response, error) {
			return c.svc.rawSearchMessages(ctx, q)
		},
	)
}

// GetUsers は停止済みユーザーを含む全ユーザーを取得する。
func (c *Client) GetUsers(ctx context.Context) ([]traq.User, error) {
	if err := c.requireScope(ScopeRead); err != nil {
		return nil, err
	}
	if v, ok := c.svc.caches.users.Get(singleCacheKey); ok {
		return v, nil
	}
	out, err := exec(ctx, c, c.svc.rawGetUsers)
	if err != nil {
		return nil, err
	}
	c.svc.caches.users.Add(singleCacheKey, out)
	return out, nil
}

// GetMyUnreadChannels は未読情報を取得する。
func (c *Client) GetMyUnreadChannels(ctx context.Context) ([]traq.UnreadChannel, error) {
	if err := c.requireScope(ScopeRead); err != nil {
		return nil, err
	}
	if v, ok := c.svc.caches.unreads.Get(singleCacheKey); ok {
		return v, nil
	}
	out, err := exec(ctx, c, c.svc.rawGetMyUnreadChannels)
	if err != nil {
		return nil, err
	}
	c.svc.caches.unreads.Add(singleCacheKey, out)
	return out, nil
}

// GetChannels はチャンネル一覧を取得する。
func (c *Client) GetChannels(ctx context.Context, includeDm bool) (*traq.ChannelList, error) {
	if err := c.requireScope(ScopeRead); err != nil {
		return nil, err
	}
	if v, ok := c.svc.caches.channels.Get(includeDm); ok {
		return v, nil
	}
	out, err := exec(ctx, c, func(ctx context.Context) (*traq.ChannelList, *http.Response, error) {
		return c.svc.rawGetChannels(ctx, includeDm)
	})
	if err != nil {
		return nil, err
	}
	c.svc.caches.channels.Add(includeDm, out)
	return out, nil
}

// GetMyChannelSubscriptions はチャンネル購読状態を取得する。
func (c *Client) GetMyChannelSubscriptions(ctx context.Context) ([]traq.UserSubscribeState, error) {
	if err := c.requireScope(ScopeRead); err != nil {
		return nil, err
	}
	if v, ok := c.svc.caches.subs.Get(singleCacheKey); ok {
		return v, nil
	}
	out, err := exec(ctx, c, c.svc.rawGetMyChannelSubscriptions)
	if err != nil {
		return nil, err
	}
	c.svc.caches.subs.Add(singleCacheKey, out)
	return out, nil
}

// GetMyStars はお気に入りチャンネルを取得する。
func (c *Client) GetMyStars(ctx context.Context) ([]string, error) {
	if err := c.requireScope(ScopeRead); err != nil {
		return nil, err
	}
	if v, ok := c.svc.caches.stars.Get(singleCacheKey); ok {
		return v, nil
	}
	out, err := exec(ctx, c, c.svc.rawGetMyStars)
	if err != nil {
		return nil, err
	}
	c.svc.caches.stars.Add(singleCacheKey, out)
	return out, nil
}

// GetStamps はスタンプ一覧を取得する。
func (c *Client) GetStamps(ctx context.Context) ([]traq.StampWithThumbnail, error) {
	if err := c.requireScope(ScopeRead); err != nil {
		return nil, err
	}
	if v, ok := c.svc.caches.stamps.Get(singleCacheKey); ok {
		return v, nil
	}
	out, err := exec(ctx, c, c.svc.rawGetStamps)
	if err != nil {
		return nil, err
	}
	c.svc.caches.stamps.Add(singleCacheKey, out)
	return out, nil
}

// GetMessage はメッセージを取得する。
func (c *Client) GetMessage(ctx context.Context, messageID string) (*traq.Message, error) {
	if err := c.requireScope(ScopeRead); err != nil {
		return nil, err
	}
	if v, ok := c.svc.caches.messages.Get(messageID); ok {
		return v, nil
	}
	out, err := exec(ctx, c, func(ctx context.Context) (*traq.Message, *http.Response, error) {
		return c.svc.rawGetMessage(ctx, messageID)
	})
	if err != nil {
		return nil, err
	}
	c.svc.caches.messages.Add(messageID, out)
	return out, nil
}

// GetFileMeta はファイルメタ情報を取得する。
func (c *Client) GetFileMeta(ctx context.Context, fileID string) (*traq.FileInfo, error) {
	if err := c.requireScope(ScopeRead); err != nil {
		return nil, err
	}
	if v, ok := c.svc.caches.fileMetas.Get(fileID); ok {
		return v, nil
	}
	out, err := exec(ctx, c, func(ctx context.Context) (*traq.FileInfo, *http.Response, error) {
		return c.svc.rawGetFileMeta(ctx, fileID)
	})
	if err != nil {
		return nil, err
	}
	c.svc.caches.fileMetas.Add(fileID, out)
	return out, nil
}

// GetFile はファイル本体を取得する。
// 返り値のファイルは呼び出し側が Close と Remove を行うこと。
func (c *Client) GetFile(ctx context.Context, fileID string) (*os.File, error) {
	if err := c.requireScope(ScopeRead); err != nil {
		return nil, err
	}
	return exec(ctx, c, func(ctx context.Context) (*os.File, *http.Response, error) {
		return c.svc.rawGetFile(ctx, fileID)
	})
}
