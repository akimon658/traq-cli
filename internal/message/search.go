package message

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
	"uuid"

	"github.com/moznion/go-optional"
	"github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/channel"
	"github.com/akimon658/traq-cli/internal/client"
	"github.com/akimon658/traq-cli/internal/config"
	"github.com/akimon658/traq-cli/internal/exit"
	"github.com/akimon658/traq-cli/internal/markdown"
	"github.com/akimon658/traq-cli/internal/session"
)

const (
	DefaultSearchLimit = 10
	MaxSearchLimit     = 100
	MaxSearchOffset    = 9900
	DefaultSearchSort  = "-createdAt"
)

// Search はメッセージを検索する。
func Search(
	ctx context.Context,
	e *config.Env,
	rt *session.ResolvedToken,
	c *client.Client,
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

	if err := session.RequireScope(rt, auth.ScopeRead); err != nil {
		return SearchResult{}, err
	}

	// ユーザー一覧は名前解決と表示の両方に使うため先に取得する。
	var users []traq.User
	if err := session.Exec(ctx, e, rt, func(ctx context.Context) (*http.Response, error) {
		var resp *http.Response
		users, resp, err = c.GetUsers(ctx)
		return resp, err
	}); err != nil {
		return SearchResult{}, err
	}
	userByID := make(map[string]traq.User, len(users))
	nameToID := make(map[string]string, len(users))
	for _, u := range users {
		userByID[u.Id] = u
		nameToID[u.Name] = u.Id
	}

	q := client.SearchQuery{
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
		channelID, err := channel.ResolveChannel(ctx, e, rt, c, opts.In)
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

	var result *traq.MessageSearchResult
	if err := session.Exec(ctx, e, rt, func(ctx context.Context) (*http.Response, error) {
		var resp *http.Response
		result, resp, err = c.SearchMessages(ctx, q)
		return resp, err
	}); err != nil {
		return SearchResult{}, err
	}

	enrich := Enrichment{
		Users: userByID, Stamps: map[string]string{},
		Quoted: map[string]traq.Message{}, Files: map[string]traq.FileInfo{},
		MyGroups: map[string]struct{}{},
	}
	enrich.SelfID, enrich.MyGroups = fetchSelfBestEffort(ctx, e, rt, c)
	if len(result.Hits) > 0 {
		var stamps []traq.StampWithThumbnail
		if err := session.Exec(ctx, e, rt, func(ctx context.Context) (*http.Response, error) {
			var resp *http.Response
			stamps, resp, err = c.GetStamps(ctx)
			return resp, err
		}); err != nil {
			return SearchResult{}, err
		}
		for _, s := range stamps {
			enrich.Stamps[s.Id] = s.Name
		}
		quoteIDs, fileIDs := markdown.CollectAllEmbedIDs(contentsOf(result.Hits), e.Origin)
		for _, id := range quoteIDs {
			var qm *traq.Message
			_ = session.Exec(ctx, e, rt, func(ctx context.Context) (*http.Response, error) {
				var resp *http.Response
				var apiErr error
				qm, resp, apiErr = c.GetMessage(ctx, id)
				return resp, apiErr
			})
			if qm != nil {
				enrich.Quoted[id] = *qm
			}
		}
		for _, id := range fileIDs {
			var fi *traq.FileInfo
			_ = session.Exec(ctx, e, rt, func(ctx context.Context) (*http.Response, error) {
				var resp *http.Response
				var apiErr error
				fi, resp, apiErr = c.GetFileMeta(ctx, id)
				return resp, apiErr
			})
			if fi != nil {
				enrich.Files[id] = *fi
			}
		}
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
		Origin:     e.Origin,
	}, nil
}

// CleanUserInputs は StringSlice の空要素を取り除く。
func CleanUserInputs(inputs []string) []string {
	out := make([]string, 0, len(inputs))
	for _, s := range inputs {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
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
