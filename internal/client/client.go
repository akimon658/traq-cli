package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/moznion/go-optional"
	"github.com/traPtitech/go-traq"
)

type Client struct {
	API    *traq.APIClient
	Origin string
}

func New(origin string) *Client {
	cfg := traq.NewConfiguration()
	cfg.Servers = traq.ServerConfigurations{{URL: origin + "/api/v3"}}
	return &Client{API: traq.NewAPIClient(cfg), Origin: origin}
}

func TokenContext(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, traq.ContextAccessToken, token)
}

type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	if e.Body != "" {
		return fmt.Sprintf("traQ API error (HTTP %d): %s", e.Status, e.Body)
	}
	return fmt.Sprintf("traQ API error (HTTP %d)", e.Status)
}

func WrapError(err error, resp *http.Response) error {
	if err == nil {
		return nil
	}
	if resp == nil {
		return err
	}
	var ge *traq.GenericOpenAPIError
	if errors.As(err, &ge) {
		return &APIError{Status: resp.StatusCode, Body: strings.TrimRight(string(ge.Body()), "\n")}
	}
	return err
}

func (c *Client) GetMe(
	ctx context.Context,
	token string,
) (*traq.MyUserDetail, *http.Response, error) {
	return c.API.MeAPI.GetMe(TokenContext(ctx, token)).Execute()
}

// MessageQuery はチャンネルメッセージ取得のパラメータ。
type MessageQuery struct {
	Limit     int32
	Offset    int32
	Since     optional.Option[time.Time]
	Until     optional.Option[time.Time]
	Inclusive bool
	Order     string
}

// GetChannelMessages はチャンネルのメッセージを取得する。トークンは ctx から解決される。
func (c *Client) GetChannelMessages(
	ctx context.Context,
	channelID string,
	q MessageQuery,
) ([]traq.Message, *http.Response, error) {
	req := c.API.MessageAPI.GetMessages(ctx, channelID).
		Limit(q.Limit).
		Offset(q.Offset).
		Inclusive(q.Inclusive).
		Order(q.Order)
	q.Since.IfSome(func(v time.Time) {
		req = req.Since(v)
	})
	q.Until.IfSome(func(v time.Time) {
		req = req.Until(v)
	})
	return req.Execute()
}

// SearchQuery はメッセージ検索 (GET /messages) のパラメータ。
type SearchQuery struct {
	Word           optional.Option[string]
	After          optional.Option[time.Time]
	Before         optional.Option[time.Time]
	In             optional.Option[string]
	To             optional.Option[[]string]
	From           optional.Option[[]string]
	Citation       optional.Option[string]
	Bot            optional.Option[bool]
	HasURL         optional.Option[bool]
	HasAttachments optional.Option[bool]
	HasImage       optional.Option[bool]
	HasVideo       optional.Option[bool]
	HasAudio       optional.Option[bool]
	Limit          int32
	Offset         int32
	Sort           string
}

// SearchMessages はメッセージを検索する。トークンは ctx から解決される。
func (c *Client) SearchMessages(
	ctx context.Context,
	q SearchQuery,
) (*traq.MessageSearchResult, *http.Response, error) {
	req := c.API.MessageAPI.SearchMessages(ctx).
		Limit(q.Limit).
		Offset(q.Offset).
		Sort(q.Sort)
	q.Word.IfSome(func(v string) {
		req = req.Word(v)
	})
	q.After.IfSome(func(v time.Time) {
		req = req.After(v)
	})
	q.Before.IfSome(func(v time.Time) {
		req = req.Before(v)
	})
	q.In.IfSome(func(v string) {
		req = req.In(v)
	})
	q.To.IfSome(func(v []string) {
		req = req.To(v)
	})
	q.From.IfSome(func(v []string) {
		req = req.From(v)
	})
	q.Citation.IfSome(func(v string) {
		req = req.Citation(v)
	})
	q.Bot.IfSome(func(v bool) {
		req = req.Bot(v)
	})
	q.HasURL.IfSome(func(v bool) {
		req = req.HasURL(v)
	})
	q.HasAttachments.IfSome(func(v bool) {
		req = req.HasAttachments(v)
	})
	q.HasImage.IfSome(func(v bool) {
		req = req.HasImage(v)
	})
	q.HasVideo.IfSome(func(v bool) {
		req = req.HasVideo(v)
	})
	q.HasAudio.IfSome(func(v bool) {
		req = req.HasAudio(v)
	})
	return req.Execute()
}

func (c *Client) GetUsers(ctx context.Context) ([]traq.User, *http.Response, error) {
	return c.API.UserAPI.GetUsers(ctx).Execute()
}

func (c *Client) GetMyUnreadChannels(
	ctx context.Context,
) ([]traq.UnreadChannel, *http.Response, error) {
	return c.API.MeAPI.GetMyUnreadChannels(ctx).Execute()
}

func (c *Client) GetChannels(
	ctx context.Context, includeDm bool,
) (*traq.ChannelList, *http.Response, error) {
	return c.API.ChannelAPI.GetChannels(ctx).IncludeDm(includeDm).Execute()
}

func (c *Client) GetMyChannelSubscriptions(
	ctx context.Context,
) ([]traq.UserSubscribeState, *http.Response, error) {
	return c.API.MeAPI.GetMyChannelSubscriptions(ctx).Execute()
}

func (c *Client) GetMyStars(
	ctx context.Context,
) ([]string, *http.Response, error) {
	return c.API.MeAPI.GetMyStars(ctx).Execute()
}

func (c *Client) GetStamps(ctx context.Context) ([]traq.StampWithThumbnail, *http.Response, error) {
	return c.API.StampAPI.GetStamps(ctx).Execute()
}

func (c *Client) GetMessage(
	ctx context.Context, messageID string,
) (*traq.Message, *http.Response, error) {
	return c.API.MessageAPI.GetMessage(ctx, messageID).Execute()
}

func (c *Client) GetFileMeta(
	ctx context.Context, fileID string,
) (*traq.FileInfo, *http.Response, error) {
	return c.API.FileAPI.GetFileMeta(ctx, fileID).Execute()
}
