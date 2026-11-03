package message

import (
	"context"

	gotraq "github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/foundation/text"
)

// FetchEnrichment はユーザー・スタンプ・引用・添付を取得する。
// ユーザー・スタンプの取得失敗はエラーにし、引用・添付の失敗は無視して続行する。
func (s *Service) FetchEnrichment(
	ctx context.Context,
	contents []string,
) (Enrichment, error) {
	out := Enrichment{
		Users:    map[string]gotraq.User{},
		Stamps:   map[string]string{},
		Quoted:   map[string]gotraq.Message{},
		Files:    map[string]gotraq.FileInfo{},
		MyGroups: map[string]struct{}{},
	}
	out.SelfID, out.MyGroups = s.fetchSelfBestEffort(ctx)
	users, err := s.fetchUserList(ctx)
	if err != nil {
		return Enrichment{}, err
	}
	for _, u := range users {
		out.Users[u.Id] = u
	}

	stamps, err := s.fetchStampMap(ctx)
	if err != nil {
		return Enrichment{}, err
	}
	out.Stamps = stamps

	out.Quoted, out.Files = s.fetchQuotesAndFiles(ctx, contents)
	return out, nil
}

// fetchUserList はユーザー一覧を取得する。
func (s *Service) fetchUserList(
	ctx context.Context,
) ([]gotraq.User, error) {
	return s.api.GetUsers(ctx)
}

// fetchStampMap はスタンプ一覧を取得し、ID->名前のマップで返す。
func (s *Service) fetchStampMap(
	ctx context.Context,
) (map[string]string, error) {
	stamps, err := s.api.GetStamps(ctx)
	if err != nil {
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
	contents []string,
) (map[string]gotraq.Message, map[string]gotraq.FileInfo) {
	quoted := map[string]gotraq.Message{}
	files := map[string]gotraq.FileInfo{}
	quoteIDs, fileIDs := text.CollectAllEmbedIDs(contents, s.origin())
	for _, id := range quoteIDs {
		if qm, err := s.api.GetMessage(ctx, id); err == nil && qm != nil {
			quoted[id] = *qm
		}
	}
	for _, id := range fileIDs {
		if fi, err := s.api.GetFileMeta(ctx, id); err == nil && fi != nil {
			files[id] = *fi
		}
	}
	return quoted, files
}

// contentsOf はメッセージ群の本文だけを抜き出す。
func contentsOf(msgs []gotraq.Message) []string {
	out := make([]string, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, m.Content)
	}
	return out
}

// fetchSelfBestEffort は自分宛メンション判定用の自IDと所属グループを取得する。
// 表示の補助情報なので、取得に失敗しても空のまま続行する。
// user.Service を使わず同一の認証済みクライアントで取得する。責務分界は docs/architecture.md を参照すること。
func (s *Service) fetchSelfBestEffort(
	ctx context.Context,
) (string, map[string]struct{}) {
	me, err := s.api.GetMe(ctx)
	if err != nil || me == nil {
		return "", map[string]struct{}{}
	}
	groups := make(map[string]struct{}, len(me.Groups))
	for _, g := range me.Groups {
		groups[g] = struct{}{}
	}
	return me.Id, groups
}
