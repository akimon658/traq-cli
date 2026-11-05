package message

import (
	"context"
	"fmt"
	"time"

	"github.com/moznion/go-optional"
	gotraq "github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/foundation/exit"
	"github.com/akimon658/traq-cli/internal/traq"
)

// List はチャンネルのメッセージ一覧を取得する。
func (s *Service) List(
	ctx context.Context,
	target string,
	opts ListOptions,
) (ListResult, error) {
	if err := ValidateListOptions(opts); err != nil {
		return ListResult{}, err
	}
	since, err := ParseRFC3339Flag("since", opts.Since)
	if err != nil {
		return ListResult{}, err
	}
	until, err := ParseRFC3339Flag("until", opts.Until)
	if err != nil {
		return ListResult{}, err
	}

	channelID, err := s.api.ResolveChannelID(ctx, target)
	if err != nil {
		return ListResult{}, err
	}

	// --unread-only では未読情報が必須なので、取得に失敗したら表示せずエラーにする。
	// 未読の境界メッセージの createdAt は unread.Since と一致するため、
	// 範囲の端は常に含めて取得する (--inclusive は無視する)。
	var (
		unread             optional.Option[gotraq.UnreadChannel]
		effectiveSince     = since
		effectiveInclusive = opts.Inclusive
	)
	if opts.UnreadOnly {
		list, err := s.api.GetMyUnreadChannels(ctx)
		if err != nil {
			return ListResult{}, err
		}
		for _, u := range list {
			if u.ChannelId != channelID {
				continue
			}
			if u.Since.IsZero() {
				return ListResult{}, exit.Generalf(
					"the unread entry of %s has no since timestamp", channelID,
				)
			}
			// --unread-only と --since は範囲の交差として新しい方を採用する。
			if sinceTime, terr := since.Take(); terr == nil && sinceTime.After(u.Since) {
				u.Since = sinceTime
			}
			effectiveSince = optional.Some(u.Since)
			effectiveInclusive = true
			unread = optional.Some(u)
			break
		}
	}

	var msgs []gotraq.Message
	// 未読が無い場合 (--unread-only の未読0件) はメッセージを取得せず表示のみ行う。
	if !opts.UnreadOnly || unread.IsSome() {
		var err error
		msgs, err = s.api.GetChannelMessages(ctx, channelID, traq.MessageQuery{
			Limit:     int32(opts.Limit),
			Offset:    int32(opts.Offset),
			Since:     effectiveSince,
			Until:     until,
			Inclusive: effectiveInclusive,
			Order:     opts.Order,
		})
		if err != nil {
			return ListResult{}, err
		}
	}

	// --show-unread 用の未読情報。表示の補助なので取得に失敗しても本体の表示は続行する。
	// --unread-only 時は取得済みなのでスキップする。
	if opts.ShowUnread && !opts.UnreadOnly {
		if list, uerr := s.api.GetMyUnreadChannels(ctx); uerr == nil {
			for i := range list {
				if list[i].ChannelId == channelID {
					unread = optional.Some(list[i])
					break
				}
			}
		}
	}

	return ListResult{
		Label:      traq.DisplayLabel(target),
		Limit:      opts.Limit,
		Offset:     opts.Offset,
		Order:      opts.Order,
		Msgs:       msgs,
		Loc:        time.Local,
		Unread:     unread,
		UnreadOnly: opts.UnreadOnly,
		Origin:     s.origin(),
	}, nil
}

// ParseRFC3339Flag は RFC3339 フラグ値を解釈する。
func ParseRFC3339Flag(name, value string) (optional.Option[time.Time], error) {
	if value == "" {
		return optional.None[time.Time](), nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return optional.None[time.Time](), exit.Usagef(
			"invalid --%s: %v (use RFC3339, e.g. 2026-01-02T15:04:05Z)", name, err,
		)
	}
	return optional.Some(t), nil
}

// SearchLabel は検索結果の表示用ラベルを組み立てる。
func SearchLabel(word, in string) string {
	label := "search"
	if word != "" {
		label += fmt.Sprintf(" %q", word)
	}
	if in != "" {
		label += fmt.Sprintf(" in %s", traq.DisplayLabel(in))
	}
	return label
}
