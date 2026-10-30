package message

import (
	"context"
	"net/http"

	"github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/domain/user"
	"github.com/akimon658/traq-cli/internal/foundation/client"
	"github.com/akimon658/traq-cli/internal/foundation/config"
	"github.com/akimon658/traq-cli/internal/foundation/text"
)

// FetchEnrichment はユーザー・スタンプ・引用・添付を取得する。
// ユーザー・スタンプの取得失敗はエラーにし、引用・添付の失敗は無視して続行する。
func FetchEnrichment(
	ctx context.Context,
	e *config.Env,
	rt *auth.ResolvedToken,
	c *client.Client,
	contents []string,
) (Enrichment, error) {
	out := Enrichment{
		Users:    map[string]traq.User{},
		Stamps:   map[string]string{},
		Quoted:   map[string]traq.Message{},
		Files:    map[string]traq.FileInfo{},
		MyGroups: map[string]struct{}{},
	}
	out.SelfID, out.MyGroups = fetchSelfBestEffort(ctx, e, rt, c)
	users, err := fetchUserList(ctx, e, rt, c)
	if err != nil {
		return Enrichment{}, err
	}
	for _, u := range users {
		out.Users[u.Id] = u
	}

	stamps, err := fetchStampMap(ctx, e, rt, c)
	if err != nil {
		return Enrichment{}, err
	}
	out.Stamps = stamps

	out.Quoted, out.Files = fetchQuotesAndFiles(ctx, e, rt, c, e.Origin, contents)
	return out, nil
}

// fetchUserList はユーザー一覧を取得する。
func fetchUserList(
	ctx context.Context,
	e *config.Env,
	rt *auth.ResolvedToken,
	c *client.Client,
) ([]traq.User, error) {
	var users []traq.User
	if err := auth.Exec(
		ctx,
		e.Origin,
		e.ClientID,
		e.LoadStore(),
		rt,
		func(ctx context.Context) (*http.Response, error) {
			var resp *http.Response
			var apiErr error
			users, resp, apiErr = c.GetUsers(ctx)
			return resp, apiErr
		},
	); err != nil {
		return nil, err
	}
	return users, nil
}

// fetchStampMap はスタンプ一覧を取得し、ID->名前のマップで返す。
func fetchStampMap(
	ctx context.Context,
	e *config.Env,
	rt *auth.ResolvedToken,
	c *client.Client,
) (map[string]string, error) {
	var stamps []traq.StampWithThumbnail
	if err := auth.Exec(
		ctx,
		e.Origin,
		e.ClientID,
		e.LoadStore(),
		rt,
		func(ctx context.Context) (*http.Response, error) {
			var resp *http.Response
			var apiErr error
			stamps, resp, apiErr = c.GetStamps(ctx)
			return resp, apiErr
		},
	); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(stamps))
	for _, s := range stamps {
		out[s.Id] = s.Name
	}
	return out, nil
}

// fetchQuotesAndFiles は引用・添付を取得する。取得失敗は無視して続行する。
func fetchQuotesAndFiles(
	ctx context.Context,
	e *config.Env,
	rt *auth.ResolvedToken,
	c *client.Client,
	origin string,
	contents []string,
) (map[string]traq.Message, map[string]traq.FileInfo) {
	quoted := map[string]traq.Message{}
	files := map[string]traq.FileInfo{}
	quoteIDs, fileIDs := text.CollectAllEmbedIDs(contents, origin)
	for _, id := range quoteIDs {
		var qm *traq.Message
		_ = auth.Exec(
			ctx,
			e.Origin,
			e.ClientID,
			e.LoadStore(),
			rt,
			func(ctx context.Context) (*http.Response, error) {
				var resp *http.Response
				var apiErr error
				qm, resp, apiErr = c.GetMessage(ctx, id)
				return resp, apiErr
			},
		)
		if qm != nil {
			quoted[id] = *qm
		}
	}
	for _, id := range fileIDs {
		var fi *traq.FileInfo
		_ = auth.Exec(
			ctx,
			e.Origin,
			e.ClientID,
			e.LoadStore(),
			rt,
			func(ctx context.Context) (*http.Response, error) {
				var resp *http.Response
				var apiErr error
				fi, resp, apiErr = c.GetFileMeta(ctx, id)
				return resp, apiErr
			},
		)
		if fi != nil {
			files[id] = *fi
		}
	}
	return quoted, files
}

// contentsOf はメッセージ群の本文だけを抜き出す。
func contentsOf(msgs []traq.Message) []string {
	out := make([]string, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, m.Content)
	}
	return out
}

// fetchSelfBestEffort は自分宛メンション判定用の自IDと所属グループを取得する。
// 表示の補助情報なので、取得に失敗しても空のまま続行する。
func fetchSelfBestEffort(
	ctx context.Context,
	e *config.Env,
	rt *auth.ResolvedToken,
	c *client.Client,
) (string, map[string]struct{}) {
	me, err := user.FetchMe(ctx, e, rt, c)
	if err != nil || me == nil {
		return "", map[string]struct{}{}
	}
	groups := make(map[string]struct{}, len(me.Groups))
	for _, g := range me.Groups {
		groups[g] = struct{}{}
	}
	return me.Id, groups
}
