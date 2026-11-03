package message

import (
	"context"
	"net/http"
	"time"

	"github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/auth"
)

// Send はチャンネルにメッセージを投稿し、表示用の付随情報と共に返す。
func (s *Service) Send(
	ctx context.Context,
	rt *auth.ResolvedToken,
	target, content string,
) (SendResult, error) {
	if err := auth.RequireScope(rt, auth.ScopeWrite); err != nil {
		return SendResult{}, err
	}
	channelID, err := s.channel.ResolveChannel(ctx, rt, target)
	if err != nil {
		return SendResult{}, err
	}
	var msg *traq.Message
	if err := s.auth.Exec(
		ctx,
		rt,
		func(ctx context.Context) (*http.Response, error) {
			var resp *http.Response
			msg, resp, err = s.client.PostMessage(ctx, channelID, content)
			return resp, err
		},
	); err != nil {
		return SendResult{}, err
	}
	enrich, err := s.FetchEnrichment(ctx, rt, []string{msg.Content})
	if err != nil {
		return SendResult{}, err
	}
	return SendResult{
		Msg:        *msg,
		Enrichment: enrich,
		Loc:        time.Local,
		Origin:     s.env.Origin,
	}, nil
}
