package message

import (
	"context"
	"net/http"
	"time"

	"github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/domain/channel"
	"github.com/akimon658/traq-cli/internal/foundation/client"
	"github.com/akimon658/traq-cli/internal/foundation/config"
)

// Send はチャンネルにメッセージを投稿し、表示用の付随情報と共に返す。
func Send(
	ctx context.Context,
	e *config.Env,
	rt *auth.ResolvedToken,
	c *client.Client,
	target, content string,
) (SendResult, error) {
	if err := auth.RequireScope(rt, auth.ScopeWrite); err != nil {
		return SendResult{}, err
	}
	channelID, err := channel.ResolveChannel(ctx, e, rt, c, target)
	if err != nil {
		return SendResult{}, err
	}
	var msg *traq.Message
	if err := auth.Exec(
		ctx,
		e.Origin,
		e.ClientID,
		e.LoadStore(),
		rt,
		func(ctx context.Context) (*http.Response, error) {
			var resp *http.Response
			msg, resp, err = c.PostMessage(ctx, channelID, content)
			return resp, err
		},
	); err != nil {
		return SendResult{}, err
	}
	enrich, err := FetchEnrichment(ctx, e, rt, c, []string{msg.Content})
	if err != nil {
		return SendResult{}, err
	}
	return SendResult{
		Msg:        *msg,
		Enrichment: enrich,
		Loc:        time.Local,
		Origin:     e.Origin,
	}, nil
}
