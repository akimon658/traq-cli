package text

import (
	"encoding/json"
	"fmt"
	"strings"

	traq "github.com/traPtitech/traq-flavored-markdown/packages/plugins/traq/go"
	markdown "github.com/traPtitech/traq-flavored-markdown/packages/sdk/go"
)

// refKind は書き換え対象ノードの種別。
type refKind int

const (
	refUser refKind = iota
	refGroup
	refChannel
	embFile
	embMessage
)

// nodeRef は書き換え判定に必要なノード情報の抜粋。
type nodeRef struct {
	kind       refKind
	start, end uint32
	id         string
}

// collectNodes は Document から参照・埋め込みノードを文書順に集める。
// コード・数式内のものはライブラリがノード化しないため、ここには現れない。
func collectNodes(doc *markdown.Document) ([]nodeRef, error) {
	if doc == nil || !doc.IsValid() {
		return nil, fmt.Errorf("text: invalid markdown document")
	}
	var out []nodeRef
	var walk func(n markdown.Node) error
	walk = func(n markdown.Node) error {
		switch n.Kind() {
		case traq.ReferenceName:
			var r traq.Reference
			if err := n.DecodePayload(&r); err != nil {
				return fmt.Errorf("text: failed to decode reference payload: %w", err)
			}
			var kind refKind
			switch r.Type {
			case traq.ReferenceKindUser:
				kind = refUser
			case traq.ReferenceKindGroup:
				kind = refGroup
			case traq.ReferenceKindChannel:
				kind = refChannel
			default:
				return fmt.Errorf("text: unknown reference kind %q", string(r.Type))
			}
			out = append(out, nodeRef{
				kind:  kind,
				start: n.Span().Start,
				end:   n.Span().End,
				id:    string(r.ID),
			})
		case traq.EmbeddingName:
			var e traq.Embedding
			if err := n.DecodePayload(&e); err != nil {
				return fmt.Errorf("text: failed to decode embedding payload: %w", err)
			}
			var kind refKind
			switch e.Type {
			case traq.EmbeddingKindFile:
				kind = embFile
			case traq.EmbeddingKindMessage:
				kind = embMessage
			default:
				return fmt.Errorf("text: unknown embedding kind %q", string(e.Type))
			}
			out = append(out, nodeRef{
				kind:  kind,
				start: n.Span().Start,
				end:   n.Span().End,
				id:    string(e.ID),
			})
		}
		for _, c := range n.Children() {
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}
	for _, c := range doc.Children() {
		if err := walk(c); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// rewriter は表示用ソース書き換えの設定を束ねる。
type rewriter struct {
	selfID      string
	groups      map[string]struct{}
	stripEmbeds bool
}

// rewrite はソースに自分宛目印の付与と引用・添付埋め込みの除去を施す。
// チャンネル参照とベア URL には触れない。
func (w rewriter) rewrite(source string, nodes []nodeRef) (string, bool, error) {
	var edits []sourceEdit
	var selfMentioned bool
	for _, n := range nodes {
		if n.end <= n.start || uint64(n.end) > uint64(len(source)) {
			return "", false, fmt.Errorf(
				"text: node span [%d,%d) out of range for source of %d bytes",
				n.start,
				n.end,
				len(source),
			)
		}
		switch n.kind {
		case refUser, refGroup:
			if !w.isSelf(n) {
				continue
			}
			literal := source[n.start:n.end]
			if !strings.HasPrefix(literal, "!") {
				// ベア参照は埋め込みを持たないため対象外 (従来の !{} 限定と同等)。
				continue
			}
			marked, err := markSelfMention(literal)
			if err != nil {
				return "", false, err
			}
			edits = append(edits, sourceEdit{start: n.start, end: n.end, text: marked})
			selfMentioned = true
		case embFile, embMessage:
			if w.stripEmbeds {
				edits = append(edits, sourceEdit{start: n.start, end: n.end, text: ""})
			}
		case refChannel:
			continue
		}
	}
	return applyEdits(source, edits), selfMentioned, nil
}

// isSelf はノードが自分宛メンションかを返す。
func (w rewriter) isSelf(n nodeRef) bool {
	switch n.kind {
	case refUser:
		return w.selfID != "" && strings.EqualFold(n.id, w.selfID)
	case refGroup:
		_, ok := w.groups[strings.ToLower(n.id)]
		return ok
	default:
		return false
	}
}

// markSelfMention は !{...} 埋め込みの raw に目印を後置する。
func markSelfMention(literal string) (string, error) {
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimPrefix(literal, "!")), &obj); err != nil {
		return "", fmt.Errorf("text: cannot parse embed JSON: %w", err)
	}
	raw, _ := obj["raw"].(string)
	obj["raw"] = raw + selfMentionMarker
	enc, err := json.Marshal(obj)
	if err != nil {
		return "", fmt.Errorf("text: cannot encode embed JSON: %w", err)
	}
	return "!" + string(enc), nil
}
