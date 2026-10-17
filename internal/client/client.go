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
