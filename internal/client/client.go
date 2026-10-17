package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

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
