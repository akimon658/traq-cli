package message

import (
	"context"
	"net/http"

	"github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/foundation/text"
)

// FetchEnrichment はユーザー・スタンプ・引用・添付を取得する。
// ユーザー・スタンプの取得失敗はエラーにし、引用・添付の失敗は無視して続行する。
func (s *Service) FetchEnrichment(
	ctx context.Context,
	rt *auth.ResolvedToken,
	contents []string,
) (Enrichment, error) {
	out := Enrichment{
		Users:    map[string]traq.User{},
		Stamps:   map[string]string{},
		Quoted:   map[string]traq.Message{},
		Files:    map[string]traq.FileInfo{},
		MyGroups: map[string]struct{}{},
	}
	out.SelfID, out.MyGroups = s.fetchSelfBestEffort(ctx, rt)
	users, err := s.fetchUserList(ctx, rt)
	if err != nil {
		return Enrichment{}, err
	}
	for _, u := range users {
		out.Users[u.Id] = u
	}

	stamps, err := s.fetchStampMap(ctx, rt)
	if err != nil {
		return Enrichment{}, err
	}
	out.Stamps = stamps

	out.Quoted, out.Files = s.fetchQuotesAndFiles(ctx, rt, contents)
	return out, nil
}

// fetchUserList はユーザー一覧を取得する。
func (s *Service) fetchUserList(
	ctx context.Context,
	rt *auth.ResolvedToken,
) ([]traq.User, error) {
	var users []traq.User
	if err := s.auth.Exec(
		ctx,
		rt,
		func(ctx context.Context) (*http.Response, error) {
			var resp *http.Response
			var apiErr error
			users, resp, apiErr = s.client.GetUsers(ctx)
			return resp, apiErr
		},
	); err != nil {
		return nil, err
	}
	return users, nil
}

// fetchStampMap はスタンプ一覧を取得し、ID->名前のマップで返す。
func (s *Service) fetchStampMap(
	ctx context.Context,
	rt *auth.ResolvedToken,
) (map[string]string, error) {
	var stamps []traq.StampWithThumbnail
	if err := s.auth.Exec(
		ctx,
		rt,
		func(ctx context.Context) (*http.Response, error) {
			var resp *http.Response
			var apiErr error
			stamps, resp, apiErr = s.client.GetStamps(ctx)
			return resp, apiErr
		},
	); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(stamps))
	for _, stamp := range stamps {
		out[stamp.Id] = stamp.Name
	}
	return out, nil
}

// fetchQuotesAndFiles は引用・添付を取得する。取得失敗は無視して続行する。
func (s *Service) fetchQuotesAndFiles(
	ctx context.Context,
	rt *auth.ResolvedToken,
	contents []string,
) (map[string]traq.Message, map[string]traq.FileInfo) {
	quoted := map[string]traq.Message{}
	files := map[string]traq.FileInfo{}
	quoteIDs, fileIDs := text.CollectAllEmbedIDs(contents, s.origin())
	for _, id := range quoteIDs {
		var qm *traq.Message
		_ = s.auth.Exec(
			ctx,
			rt,
			func(ctx context.Context) (*http.Response, error) {
				var resp *http.Response
				var apiErr error
				qm, resp, apiErr = s.client.GetMessage(ctx, id)
				return resp, apiErr
			},
		)
		if qm != nil {
			quoted[id] = *qm
		}
	}
	for _, id := range fileIDs {
		var fi *traq.FileInfo
		_ = s.auth.Exec(
			ctx,
			rt,
			func(ctx context.Context) (*http.Response, error) {
				var resp *http.Response
				var apiErr error
				fi, resp, apiErr = s.client.GetFileMeta(ctx, id)
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
// user.Service を使わず同一の auth/client で取得する。責務分界は docs/architecture.md を参照すること。
func (s *Service) fetchSelfBestEffort(
	ctx context.Context,
	rt *auth.ResolvedToken,
) (string, map[string]struct{}) {
	if err := auth.RequireScope(rt, auth.ScopeRead); err != nil {
		return "", map[string]struct{}{}
	}
	var me *traq.MyUserDetail
	if err := s.auth.Exec(
		ctx,
		rt,
		func(ctx context.Context) (*http.Response, error) {
			var resp *http.Response
			var apiErr error
			me, resp, apiErr = s.client.GetMe(ctx, rt.Token.AccessToken)
			return resp, apiErr
		},
	); err != nil || me == nil {
		return "", map[string]struct{}{}
	}
	groups := make(map[string]struct{}, len(me.Groups))
	for _, g := range me.Groups {
		groups[g] = struct{}{}
	}
	return me.Id, groups
}
