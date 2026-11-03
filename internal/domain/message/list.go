package message

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/moznion/go-optional"
	"github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/domain/channel"
	"github.com/akimon658/traq-cli/internal/foundation/client"
	"github.com/akimon658/traq-cli/internal/foundation/exit"
)

// List はチャンネルのメッセージ一覧を取得する。
func (s *Service) List(
	ctx context.Context,
	rt *auth.ResolvedToken,
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

	if err := auth.RequireScope(rt, auth.ScopeRead); err != nil {
		return ListResult{}, err
	}
	channelID, err := s.channel.ResolveChannel(ctx, rt, target)
	if err != nil {
		return ListResult{}, err
	}

	// --unread-only では未読情報が必須なので、取得に失敗したら表示せずエラーにする。
	// 未読の境界メッセージの createdAt は unread.Since と一致するため、
	// 範囲の端は常に含めて取得する (--inclusive は無視する)。
	var (
		unread             optional.Option[traq.UnreadChannel]
		effectiveSince     = since
		effectiveInclusive = opts.Inclusive
	)
	if opts.UnreadOnly {
		var list []traq.UnreadChannel
		if err := s.auth.Exec(
			ctx,
			rt,
			func(ctx context.Context) (*http.Response, error) {
				var resp *http.Response
				var apiErr error
				list, resp, apiErr = s.client.GetMyUnreadChannels(ctx)
				return resp, apiErr
			},
		); err != nil {
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

	var msgs []traq.Message
	// 未読が無い場合 (--unread-only の未読0件) はメッセージを取得せず表示のみ行う。
	if !opts.UnreadOnly || unread.IsSome() {
		if err := s.auth.Exec(
			ctx,
			rt,
			func(ctx context.Context) (*http.Response, error) {
				var resp *http.Response
				msgs, resp, err = s.client.GetChannelMessages(ctx, channelID, client.MessageQuery{
					Limit:     int32(opts.Limit),
					Offset:    int32(opts.Offset),
					Since:     effectiveSince,
					Until:     until,
					Inclusive: effectiveInclusive,
					Order:     opts.Order,
				})
				return resp, err
			},
		); err != nil {
			return ListResult{}, err
		}
	}

	enrich := Enrichment{
		Users: map[string]traq.User{}, Stamps: map[string]string{},
		Quoted: map[string]traq.Message{}, Files: map[string]traq.FileInfo{},
		MyGroups: map[string]struct{}{},
	}
	if len(msgs) > 0 {
		enrich, err = s.FetchEnrichment(ctx, rt, contentsOf(msgs))
		if err != nil {
			return ListResult{}, err
		}
	}

	// --show-unread 用の未読情報。表示の補助なので取得に失敗しても本体の表示は続行する。
	// --unread-only 時は取得済みなのでスキップする。
	if opts.ShowUnread && !opts.UnreadOnly {
		var list []traq.UnreadChannel
		uerr := s.auth.Exec(
			ctx,
			rt,
			func(ctx context.Context) (*http.Response, error) {
				var resp *http.Response
				var apiErr error
				list, resp, apiErr = s.client.GetMyUnreadChannels(ctx)
				return resp, apiErr
			},
		)
		if uerr == nil {
			for i := range list {
				if list[i].ChannelId == channelID {
					unread = optional.Some(list[i])
					break
				}
			}
		}
	}

	return ListResult{
		Label:      channel.DisplayLabel(target),
		Limit:      opts.Limit,
		Offset:     opts.Offset,
		Order:      opts.Order,
		Msgs:       msgs,
		Enrichment: enrich,
		Loc:        time.Local,
		Unread:     unread,
		UnreadOnly: opts.UnreadOnly,
		Origin:     s.env.Origin,
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
		label += fmt.Sprintf(" in %s", channel.DisplayLabel(in))
	}
	return label
}
