package message

import (
	"context"
	"fmt"
	"strings"
	"time"
	"uuid"

	"github.com/moznion/go-optional"
	gotraq "github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/foundation/exit"
	"github.com/akimon658/traq-cli/internal/traq"
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

	// ユーザー一覧は名前解決と表示の両方に使うため先に取得する。
	users, err := s.fetchUserList(ctx)
	if err != nil {
		return SearchResult{}, err
	}
	userByID := make(map[string]gotraq.User, len(users))
	nameToID := make(map[string]string, len(users))
	for _, u := range users {
		userByID[u.Id] = u
		nameToID[u.Name] = u.Id
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
		channelID, err := s.channel.ResolveChannel(ctx, opts.In)
		if err != nil {
			return SearchResult{}, err
		}
		q.In = optional.Some(channelID)
	}
	if from := CleanUserInputs(opts.From); len(from) > 0 {
		ids, err := ResolveUserIDs(from, nameToID)
		if err != nil {
			return SearchResult{}, exit.Usagef("invalid --from: %v", err)
		}
		q.From = optional.Some(ids)
	}
	if to := CleanUserInputs(opts.To); len(to) > 0 {
		ids, err := ResolveUserIDs(to, nameToID)
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

	enrich := Enrichment{
		Users: userByID, Stamps: map[string]string{},
		Quoted: map[string]gotraq.Message{}, Files: map[string]gotraq.FileInfo{},
		MyGroups: map[string]struct{}{},
	}
	enrich.SelfID, enrich.MyGroups = s.fetchSelfBestEffort(ctx)
	if len(result.Hits) > 0 {
		stamps, err := s.fetchStampMap(ctx)
		if err != nil {
			return SearchResult{}, err
		}
		enrich.Stamps = stamps
		enrich.Quoted, enrich.Files = s.fetchQuotesAndFiles(
			ctx, contentsOf(result.Hits),
		)
	}

	return SearchResult{
		Label:      SearchLabel(word, opts.In),
		Limit:      opts.Limit,
		Offset:     opts.Offset,
		Sort:       opts.Sort,
		TotalHits:  result.TotalHits,
		Msgs:       result.Hits,
		Enrichment: enrich,
		Loc:        time.Local,
		Origin:     s.origin(),
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

// ResolveUserIDs は UUID または @name/name を UUID に変換する。
func ResolveUserIDs(inputs []string, nameToID map[string]string) ([]string, error) {
	ids := make([]string, 0, len(inputs))
	for _, in := range inputs {
		if _, err := uuid.Parse(in); err == nil {
			ids = append(ids, in)
			continue
		}
		name := strings.TrimPrefix(in, "@")
		if name == "" {
			return nil, fmt.Errorf("empty user name")
		}
		id, ok := nameToID[name]
		if !ok {
			return nil, fmt.Errorf("user not found: %s", in)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
