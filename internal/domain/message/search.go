package message

import (
	"context"
	"strings"

	"github.com/moznion/go-optional"

	"github.com/akimon658/traq-cli/internal/domain/traq"
	"github.com/akimon658/traq-cli/internal/foundation/exit"
)

const (
	DefaultSearchLimit = 10
	MaxSearchLimit     = 100
	MaxSearchOffset    = 9900
	DefaultSearchSort  = "-createdAt"
)

// Search はメッセージを検索する。
func (s *Service) Search(
	ctx context.Context,
	word string,
	opts SearchOptions,
) (SearchResult, error) {
	if err := ValidateSearchOptions(word, opts); err != nil {
		return SearchResult{}, err
	}
	after, err := ParseRFC3339Flag("after", opts.After)
	if err != nil {
		return SearchResult{}, err
	}
	before, err := ParseRFC3339Flag("before", opts.Before)
	if err != nil {
		return SearchResult{}, err
	}

	q := traq.SearchQuery{
		After:          after,
		Before:         before,
		Bot:            opts.Bot,
		HasURL:         opts.HasURL,
		HasAttachments: opts.HasAttachments,
		HasImage:       opts.HasImage,
		HasVideo:       opts.HasVideo,
		HasAudio:       opts.HasAudio,
		Limit:          int32(opts.Limit),
		Offset:         int32(opts.Offset),
		Sort:           opts.Sort,
	}
	if word != "" {
		q.Word = optional.Some(word)
	}
	if opts.In != "" {
		channelID, err := s.api.ResolveChannelID(ctx, opts.In)
		if err != nil {
			return SearchResult{}, err
		}
		q.In = optional.Some(channelID)
	}
	if from := CleanUserInputs(opts.From); len(from) > 0 {
		ids, err := s.api.ResolveUserIDs(ctx, from)
		if err != nil {
			return SearchResult{}, exit.Usagef("invalid --from: %v", err)
		}
		q.From = optional.Some(ids)
	}
	if to := CleanUserInputs(opts.To); len(to) > 0 {
		ids, err := s.api.ResolveUserIDs(ctx, to)
		if err != nil {
			return SearchResult{}, exit.Usagef("invalid --to: %v", err)
		}
		q.To = optional.Some(ids)
	}
	if opts.Citation != "" {
		q.Citation = optional.Some(opts.Citation)
	}

	result, err := s.api.SearchMessages(ctx, q)
	if err != nil {
		return SearchResult{}, err
	}

	return SearchResult{
		Label:     SearchLabel(word, opts.In),
		Limit:     opts.Limit,
		Offset:    opts.Offset,
		Sort:      opts.Sort,
		TotalHits: result.TotalHits,
		Msgs:      result.Hits,
	}, nil
}

// CleanUserInputs は StringSlice の空要素を取り除く。
func CleanUserInputs(inputs []string) []string {
	out := make([]string, 0, len(inputs))
	for _, str := range inputs {
		str = strings.TrimSpace(str)
		if str != "" {
			out = append(out, str)
		}
	}
	return out
}
