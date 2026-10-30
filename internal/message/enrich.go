package message

import (
	"context"
	"net/http"

	"github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/client"
	"github.com/akimon658/traq-cli/internal/config"
	"github.com/akimon658/traq-cli/internal/markdown"
	"github.com/akimon658/traq-cli/internal/session"
)

// FetchEnrichment はユーザー・スタンプ・引用・添付を取得する。
// ユーザー・スタンプの取得失敗はエラーにし、引用・添付の失敗は無視して続行する。
func FetchEnrichment(
	ctx context.Context,
	e *config.Env,
	rt *session.ResolvedToken,
	c *client.Client,
	contents []string,
) (Enrichment, error) {
	out := Enrichment{
		Users:  map[string]traq.User{},
		Stamps: map[string]string{},
		Quoted: map[string]traq.Message{},
		Files:  map[string]traq.FileInfo{},
	}
	var users []traq.User
	if err := session.Exec(ctx, e, rt, func(ctx context.Context) (*http.Response, error) {
		var resp *http.Response
		var apiErr error
		users, resp, apiErr = c.GetUsers(ctx)
		return resp, apiErr
	}); err != nil {
		return Enrichment{}, err
	}
	for _, u := range users {
		out.Users[u.Id] = u
	}

	var stamps []traq.StampWithThumbnail
	if err := session.Exec(ctx, e, rt, func(ctx context.Context) (*http.Response, error) {
		var resp *http.Response
		var apiErr error
		stamps, resp, apiErr = c.GetStamps(ctx)
		return resp, apiErr
	}); err != nil {
		return Enrichment{}, err
	}
	for _, s := range stamps {
		out.Stamps[s.Id] = s.Name
	}

	quoteIDs, fileIDs := markdown.CollectAllEmbedIDs(contents, e.Origin)
	for _, id := range quoteIDs {
		var qm *traq.Message
		_ = session.Exec(ctx, e, rt, func(ctx context.Context) (*http.Response, error) {
			var resp *http.Response
			var apiErr error
			qm, resp, apiErr = c.GetMessage(ctx, id)
			return resp, apiErr
		})
		if qm != nil {
			out.Quoted[id] = *qm
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
			out.Files[id] = *fi
		}
	}
	return out, nil
}

// contentsOf はメッセージ群の本文だけを抜き出す。
func contentsOf(msgs []traq.Message) []string {
	out := make([]string, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, m.Content)
	}
	return out
}
