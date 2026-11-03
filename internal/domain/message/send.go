package message

import (
	"context"
	"time"
)

// Send はチャンネルにメッセージを投稿し、表示用の付随情報と共に返す。
func (s *Service) Send(
	ctx context.Context,
	target, content string,
) (SendResult, error) {
	channelID, err := s.channel.ResolveChannel(ctx, target)
	if err != nil {
		return SendResult{}, err
	}
	msg, err := s.api.PostMessage(ctx, channelID, content)
	if err != nil {
		return SendResult{}, err
	}
	enrich, err := s.FetchEnrichment(ctx, []string{msg.Content})
	if err != nil {
		return SendResult{}, err
	}
	return SendResult{
		Msg:        *msg,
		Enrichment: enrich,
		Loc:        time.Local,
		Origin:     s.origin(),
	}, nil
}
