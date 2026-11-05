package message

import (
	"context"

	gotraq "github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/foundation/text"
)

// GetEnrichment はユーザー・スタンプ・引用・添付を取得する。
// ユーザー・スタンプの取得失敗はエラーにし、引用・添付の失敗は無視して続行する。
func (s *Service) GetEnrichment(
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
	// 自分宛メンション判定用の補助情報なので、取得失敗時は空のまま続行する。
	selfID, groups, _ := s.api.GetSelfIDAndGroups(ctx)
	out.SelfID = selfID
	out.MyGroups = groups
	users, err := s.api.GetUsers(ctx)
	if err != nil {
		return Enrichment{}, err
	}
	for _, u := range users {
		out.Users[u.Id] = u
	}

	stamps, err := s.api.GetStampNameMap(ctx)
	if err != nil {
		return Enrichment{}, err
	}
	out.Stamps = stamps

	out.Quoted, out.Files = s.getQuotesAndFiles(ctx, contents)
	return out, nil
}

// getQuotesAndFiles は引用・添付を取得する。取得失敗は無視して続行する。
func (s *Service) getQuotesAndFiles(
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
