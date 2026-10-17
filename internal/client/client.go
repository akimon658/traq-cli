package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

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
	Since     *time.Time
	Until     *time.Time
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
	if q.Since != nil {
		req = req.Since(*q.Since)
	}
	if q.Until != nil {
		req = req.Until(*q.Until)
	}
	return req.Execute()
}

// SearchQuery はメッセージ検索 (GET /messages) のパラメータ。
// ポインタのフィールドは未指定 (API に送らない) を表す。
type SearchQuery struct {
	Word           string
	After          *time.Time
	Before         *time.Time
	In             string
	InSet          bool
	To             []string
	ToSet          bool
	From           []string
	FromSet        bool
	Citation       string
	CitationSet    bool
	Bot            *bool
	HasURL         *bool
	HasAttachments *bool
	HasImage       *bool
	HasVideo       *bool
	HasAudio       *bool
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
	if q.Word != "" {
		req = req.Word(q.Word)
	}
	if q.After != nil {
		req = req.After(*q.After)
	}
	if q.Before != nil {
		req = req.Before(*q.Before)
	}
	if q.InSet {
		req = req.In(q.In)
	}
	if q.ToSet {
		req = req.To(q.To)
	}
	if q.FromSet {
		req = req.From(q.From)
	}
	if q.CitationSet {
		req = req.Citation(q.Citation)
	}
	if q.Bot != nil {
		req = req.Bot(*q.Bot)
	}
	if q.HasURL != nil {
		req = req.HasURL(*q.HasURL)
	}
	if q.HasAttachments != nil {
		req = req.HasAttachments(*q.HasAttachments)
	}
	if q.HasImage != nil {
		req = req.HasImage(*q.HasImage)
	}
	if q.HasVideo != nil {
		req = req.HasVideo(*q.HasVideo)
	}
	if q.HasAudio != nil {
		req = req.HasAudio(*q.HasAudio)
	}
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

func (c *Client) GetStamps(ctx context.Context) ([]traq.StampWithThumbnail, *http.Response, error) {
	return c.API.StampAPI.GetStamps(ctx).Execute()
}
