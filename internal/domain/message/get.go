package message

import (
	"context"
)

// Get はメッセージを1件取得し、表示用の付随情報と共に返す。
func (s *Service) Get(
	ctx context.Context,
	messageID string,
) (GetResult, error) {
	if err := ValidateMessageID(messageID); err != nil {
		return GetResult{}, err
	}
	msg, err := s.api.GetMessage(ctx, messageID)
	if err != nil {
		return GetResult{}, err
	}
	return GetResult{
		Msg:          *msg,
		ChannelLabel: s.api.GetChannelLabel(ctx, msg.ChannelId),
	}, nil
}
