package view

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/moznion/go-optional"
	"github.com/traPtitech/go-traq"

	itraq "github.com/akimon658/traq-cli/internal/domain/traq"
	"github.com/akimon658/traq-cli/internal/foundation/text"
)

// MessageRenderer はメッセージ表示に必要な遅延解決を束ねる。
// Client はコンストラクタで固定し、要求ごとの値はメソッド引数で受ける。
// ユーザー・スタンプ・引用・添付の解決失敗はIDフォールバック表示に倒し、全体エラーにしない。
type MessageRenderer struct {
	client itraq.Client
	loc    *time.Location
	origin string
}

// NewMessageRenderer はメッセージ表示器を作る。
// loc が nil なら time.Local、origin は client.Origin() を正典とする。
func NewMessageRenderer(client itraq.Client, loc *time.Location) *MessageRenderer {
	if loc == nil {
		loc = time.Local
	}
	var origin string
	if client != nil {
		origin = client.Origin()
	}
	return &MessageRenderer{client: client, loc: loc, origin: origin}
}

// MessageList はメッセージ表示に必要な情報をまとめた DTO。
// list / search で共有する。ListViewParams.Build() 等経由での構築を推奨する。
type MessageList struct {
	Label  string
	Limit  int
	Offset int
	Order  string
	Msgs   []traq.Message
	Unread optional.Option[traq.UnreadChannel]
	// UnreadOnly が true なら未読メッセージのみを表示するモード。
	// Unread が None のときは未読0件として出力する。
	UnreadOnly bool
	// TotalHits が Some なら検索結果として表示する。
	TotalHits optional.Option[int64]
	Sort      string
}

// ListViewParams は list / unread-only 表示用の入力。
type ListViewParams struct {
	Label      string
	Limit      int
	Offset     int
	Order      string
	Msgs       []traq.Message
	Unread     optional.Option[traq.UnreadChannel]
	UnreadOnly bool
}

// SearchViewParams は search 表示用の入力。
type SearchViewParams struct {
	Label     string
	Limit     int
	Offset    int
	Sort      string
	TotalHits int64
	Msgs      []traq.Message
}

// SingleViewParams は send の1件表示用の入力。
type SingleViewParams struct {
	Msg traq.Message
}

// GetViewParams は get の1件表示用の入力。先頭に投稿先チャンネルのラベル行を出す。
type GetViewParams struct {
	Label string
	Msg   traq.Message
}

// Build は ListViewParams から MessageList を作る。
func (p ListViewParams) Build() MessageList {
	return MessageList{
		Label: p.Label, Limit: p.Limit, Offset: p.Offset, Order: p.Order,
		Msgs: p.Msgs, Unread: p.Unread, UnreadOnly: p.UnreadOnly,
	}
}

// Build は SearchViewParams から MessageList を作る。
func (p SearchViewParams) Build() MessageList {
	return MessageList{
		Label: p.Label, Limit: p.Limit, Offset: p.Offset, Sort: p.Sort,
		TotalHits: optional.Some(p.TotalHits),
		Msgs:      p.Msgs,
	}
}

// WriteMessageList はメッセージ一覧を可読性の高いテキストで出力する。
// 取得順 (order) に関わらず、表示は常に時系列 (古い順) に並べる。
func (r *MessageRenderer) WriteMessageList(
	ctx context.Context,
	w io.Writer,
	ml MessageList,
) error {
	msgs := make([]traq.Message, len(ml.Msgs))
	copy(msgs, ml.Msgs)
	sort.SliceStable(msgs, func(i, j int) bool {
		if !msgs[i].CreatedAt.Equal(msgs[j].CreatedAt) {
			return msgs[i].CreatedAt.Before(msgs[j].CreatedAt)
		}
		return msgs[i].Id < msgs[j].Id
	})

	if total, err := ml.TotalHits.Take(); err == nil {
		if _, err := fmt.Fprintf(
			w, "%s (全%d件中 %d件 limit=%d offset=%d sort=%s)\n\n",
			ml.Label, total, len(msgs), ml.Limit, ml.Offset, ml.Sort,
		); err != nil {
			return err
		}
	} else if ml.UnreadOnly {
		if unread, uerr := ml.Unread.Take(); uerr == nil {
			if _, err := fmt.Fprintf(
				w, "%s (未読%d件中 %d件 limit=%d offset=%d order=%s)\n\n",
				ml.Label, unread.Count, len(msgs), ml.Limit, ml.Offset, ml.Order,
			); err != nil {
				return err
			}
		} else if _, err := fmt.Fprintf(w, "%s (未読0件)\n\n", ml.Label); err != nil {
			return err
		}
	} else if _, err := fmt.Fprintf(
		w, "%s (%d件 limit=%d offset=%d order=%s)\n\n",
		ml.Label, len(msgs), ml.Limit, ml.Offset, ml.Order,
	); err != nil {
		return err
	}

	unread, uerr := ml.Unread.Take()
	divider := ""
	if uerr == nil && !ml.UnreadOnly {
		divider = UnreadDivider(unread)
	}
	dividerAt := -1
	if divider != "" {
		for i, m := range msgs {
			if m.Id == unread.OldestMessageId {
				dividerAt = i
				break
			}
		}
	}

	var blocks []string
	for i, msg := range msgs {
		if i == dividerAt {
			blocks = append(blocks, divider)
		}
		blocks = append(blocks, r.formatMessage(ctx, msg))
	}
	// 未読はあるが、その先頭がこのページに含まれない場合は注記だけ出す。
	if divider != "" && dividerAt < 0 {
		blocks = append(
			[]string{fmt.Sprintf("# 未読 %d 件 (このページに未読の先頭は含まれない)", unread.Count)},
			blocks...,
		)
	}
	if len(blocks) > 0 {
		if _, err := fmt.Fprintln(w, strings.Join(blocks, "\n\n")); err != nil {
			return err
		}
	}
	return nil
}

// WriteSingleMessage はメッセージ1件を list/search と同じ形式で出力する。
func (r *MessageRenderer) WriteSingleMessage(
	ctx context.Context,
	w io.Writer,
	params SingleViewParams,
) error {
	_, err := fmt.Fprintln(w, r.formatMessage(ctx, params.Msg))
	return err
}

// writeChannelLabel はチャンネルのラベル行だけを出力する。
func writeChannelLabel(w io.Writer, label string) error {
	_, err := fmt.Fprintln(w, label)
	return err
}

// WriteGetMessage はチャンネルラベル行の後にメッセージ1件を出力する。
func (r *MessageRenderer) WriteGetMessage(
	ctx context.Context,
	w io.Writer,
	params GetViewParams,
) error {
	if err := writeChannelLabel(w, params.Label); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	return r.WriteSingleMessage(ctx, w, SingleViewParams{Msg: params.Msg})
}

// UnreadDivider は未読境界の見出しを返す。
func UnreadDivider(u traq.UnreadChannel) string {
	if u.OldestMessageId == "" {
		return ""
	}
	label := fmt.Sprintf("ここから未読 (%d件)", u.Count)
	if u.Noticeable {
		label += " [mentionあり]"
	}
	return "----- " + label + " -----"
}

// selfInfo は自分宛メンション判定用の自IDと所属グループを返す。
// 取得失敗時は空を返し、ハイライトなしとして続行する。
func (r *MessageRenderer) selfInfo(ctx context.Context) (string, map[string]struct{}) {
	if r == nil || r.client == nil {
		return "", nil
	}
	selfID, groups, err := r.client.GetSelfIDAndGroups(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning: failed to resolve self info:", err)
		return "", nil
	}
	return selfID, groups
}

// userDisplay はユーザーIDから表示用ラベルを返す。
// 解決失敗時はIDフォールバック表示に倒す。
func (r *MessageRenderer) userDisplay(ctx context.Context, id string) string {
	if r == nil || r.client == nil {
		return UserFallbackLabel(id)
	}
	u, err := r.client.GetUserByID(ctx, id)
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning: failed to resolve user:", err)
		return UserFallbackLabel(id)
	}
	return UserLabel(u)
}

// stampName はスタンプIDから名前を返す。解決失敗時はIDをそのまま返す。
func (r *MessageRenderer) stampName(ctx context.Context, id string) string {
	if r == nil || r.client == nil {
		return id
	}
	name, err := r.client.GetStampName(ctx, id)
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning: failed to resolve stamp:", err)
		return id
	}
	return name
}

// formatMessage はメッセージ1件を表示用ブロックに整形する。
// 解決失敗はフォールバック表示に倒すため、エラーを返さない。
func (r *MessageRenderer) formatMessage(ctx context.Context, msg traq.Message) string {
	loc := time.Local
	if r != nil && r.loc != nil {
		loc = r.loc
	}
	origin := ""
	if r != nil {
		origin = r.origin
	}
	header := fmt.Sprintf(
		"%s %s id:%s",
		msg.CreatedAt.In(loc).Format(time.RFC3339),
		r.userDisplay(ctx, msg.UserId),
		msg.Id,
	)
	if !msg.UpdatedAt.Equal(msg.CreatedAt) {
		header += " [edited]"
	}
	if msg.Pinned {
		header += " [pinned]"
	}

	selfID, groups := r.selfInfo(ctx)
	clean := text.CleanContentWithHighlight(msg.Content, origin, selfID, groups)
	if clean.SelfMentioned {
		header += " [to you]"
	}

	lines := []string{header}
	if clean.Body != "" {
		for _, line := range strings.Split(clean.Body, "\n") {
			if line == "" {
				lines = append(lines, "")
			} else {
				lines = append(lines, "  "+line)
			}
		}
	}
	for _, qid := range clean.QuoteIDs {
		lines = append(lines, r.formatQuote(ctx, qid)...)
	}
	if len(clean.FileIDs) > 0 {
		lines = append(lines, "添付ファイル")
		for _, fid := range clean.FileIDs {
			lines = append(lines, "  "+r.formatFile(ctx, fid))
		}
	}
	if items := r.formatStampItems(ctx, msg.Stamps); len(items) > 0 {
		lines = append(lines, "スタンプ")
		for _, it := range items {
			lines = append(lines, "  "+it)
		}
	}
	return strings.Join(lines, "\n")
}

// formatQuote は引用1件分の行群を返す。取得失敗時はヘッダー1行のみ。
func (r *MessageRenderer) formatQuote(ctx context.Context, quoteID string) []string {
	if r == nil || r.client == nil {
		return []string{fmt.Sprintf("引用 (取得失敗 id:%s)", quoteID)}
	}
	qm, err := r.client.GetMessage(ctx, quoteID)
	if err != nil || qm == nil {
		if err != nil {
			fmt.Fprintln(os.Stderr, "warning: failed to resolve quoted message:", err)
		} else {
			fmt.Fprintln(
				os.Stderr,
				"warning: failed to resolve quoted message: nil message id:"+quoteID,
			)
		}
		return []string{fmt.Sprintf("引用 (取得失敗 id:%s)", quoteID)}
	}
	loc := time.Local
	if r.loc != nil {
		loc = r.loc
	}
	label := r.userDisplay(ctx, qm.UserId)
	header := fmt.Sprintf(
		"引用 %s %s id:%s",
		label,
		qm.CreatedAt.In(loc).Format(time.RFC3339),
		qm.Id,
	)
	if !qm.UpdatedAt.Equal(qm.CreatedAt) {
		header += " [edited]"
	}
	lines := []string{header}
	selfID, groups := r.selfInfo(ctx)
	if body := text.RenderQuoteBodyWithHighlight(qm.Content, selfID, groups); body != "" {
		for _, line := range strings.Split(body, "\n") {
			if line == "" {
				lines = append(lines, "")
			} else {
				lines = append(lines, "  "+line)
			}
		}
	}
	return lines
}

// formatFile は添付ファイル1件の表示行を返す。取得失敗時はフォールバック表示。
func (r *MessageRenderer) formatFile(ctx context.Context, id string) string {
	if r == nil || r.client == nil {
		return fmt.Sprintf("(取得失敗 id:%s)", id)
	}
	fi, err := r.client.GetFileMeta(ctx, id)
	if err != nil || fi == nil {
		if err != nil {
			fmt.Fprintln(os.Stderr, "warning: failed to resolve file metadata:", err)
		} else {
			fmt.Fprintln(os.Stderr, "warning: failed to resolve file metadata: nil metadata id:"+id)
		}
		return fmt.Sprintf("(取得失敗 id:%s)", id)
	}
	return fmt.Sprintf("%s (%s, %d bytes, id:%s)", fi.Name, fi.Mime, fi.Size, fi.Id)
}

// formatStampItems はスタンプを stampId で集約し、最初に押された順
// (traQ_S-UI の createStampList と同等) に ":name: ×sum" の一覧で返す。
// 名前解決の失敗はIDフォールバックに倒すため、エラーを返さない。
func (r *MessageRenderer) formatStampItems(
	ctx context.Context,
	stamps []traq.MessageStamp,
) []string {
	if len(stamps) == 0 {
		return nil
	}
	type aggregated struct {
		stampID    string
		sum        int32
		createdAt  time.Time
		firstIndex int
	}
	byID := make(map[string]*aggregated, len(stamps))
	order := make([]*aggregated, 0, len(stamps))
	for i, s := range stamps {
		a, ok := byID[s.StampId]
		if !ok {
			a = &aggregated{stampID: s.StampId, createdAt: s.CreatedAt, firstIndex: i}
			byID[s.StampId] = a
			order = append(order, a)
		} else if s.CreatedAt.Before(a.createdAt) {
			a.createdAt = s.CreatedAt
		}
		a.sum += s.Count
	}
	sort.SliceStable(order, func(i, j int) bool {
		if !order[i].createdAt.Equal(order[j].createdAt) {
			return order[i].createdAt.Before(order[j].createdAt)
		}
		return order[i].firstIndex < order[j].firstIndex
	})
	parts := make([]string, 0, len(order))
	for _, a := range order {
		name := r.stampName(ctx, a.stampID)
		parts = append(parts, fmt.Sprintf(":%s: ×%d", name, a.sum))
	}
	return parts
}
