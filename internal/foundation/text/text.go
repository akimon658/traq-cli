// Package text は traq-flavored-markdown を用いたメッセージ本文解析のファサード。
// 埋め込み・URL の解釈はライブラリに任せ、当パッケージは自分宛メンションの目印付与と
// 引用・添付埋め込みの表示用除去（ソース書き換え）だけを受け持つ。
package text

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	markdown "github.com/traPtitech/traq-flavored-markdown/packages/sdk/go"
)

// selfMentionMarker は自分宛メンションに後置する目印。
const selfMentionMarker = " [to you]"

// Parsed はメッセージ本文1件の解析結果。
type Parsed struct {
	// Body は表示用本文 (ライブラリの MessageText に目印付与・埋め込み除去を施したもの)。
	Body string
	// QuoteIDs は出現順の引用メッセージID。
	QuoteIDs []string
	// FileIDs は出現順のファイルID。
	FileIDs []string
	// SelfMentioned はトップレベル本文に自分宛メンションを含むか。
	SelfMentioned bool
}

// ParseInput は Parser への入力を束ねる。
type ParseInput struct {
	Content string
	Origin  string
	SelfID  string
	Groups  map[string]struct{}
}

// Parser はメッセージ本文を解析する。
//
//go:generate go tool mockgen -destination=mock_parser.go -package=text github.com/akimon658/traq-cli/internal/foundation/text Parser
type Parser interface {
	// Parse はトップレベル本文用に解析する。
	// 引用・添付の !{} 埋め込みは本文から取り除き、ID だけを返す。
	// 自分宛メンションには目印を後置する。
	Parse(ctx context.Context, in ParseInput) (Parsed, error)
	// ParseQuote は引用本文 (1段目) 用に解析する。
	// 入れ子の引用・添付は本文中のラベル ([引用メッセージ]/[添付ファイル]) のまま残し、
	// 取得は行わない。ID は参考用に返すが、呼び出し側は無視してよい。
	ParseQuote(ctx context.Context, in ParseInput) (Parsed, error)
	// Close は保持するランタイムを解放する。
	Close(ctx context.Context) error
}

// processor は1回の処理呼び出しを表す。テストで差し替えるための境界。
type processor interface {
	Process(ctx context.Context, source string) (*markdown.ProcessedDocument, error)
}

// WASMParser は traq-flavored-markdown の WASM ランタイムを用いた Parser 実装。
type WASMParser struct {
	runtime    *markdown.Runtime
	mu         sync.Mutex
	processors map[string]*markdown.Processor
}

// NewWASMParser はバンドルされた WASM ランタイムで Parser を作る。
func NewWASMParser(ctx context.Context) (*WASMParser, error) {
	rt, err := markdown.NewBundledRuntime(ctx)
	if err != nil {
		return nil, fmt.Errorf("text: failed to create markdown runtime: %w", err)
	}
	return &WASMParser{runtime: rt, processors: make(map[string]*markdown.Processor)}, nil
}

// Parse はトップレベル本文用に解析する。
func (p *WASMParser) Parse(ctx context.Context, in ParseInput) (Parsed, error) {
	return p.parse(ctx, in, true)
}

// ParseQuote は引用本文 (1段目) 用に解析する。
func (p *WASMParser) ParseQuote(ctx context.Context, in ParseInput) (Parsed, error) {
	return p.parse(ctx, in, false)
}

func (p *WASMParser) parse(
	ctx context.Context,
	in ParseInput,
	stripEmbeds bool,
) (Parsed, error) {
	proc, err := p.processorFor(ctx, in.Origin)
	if err != nil {
		return Parsed{}, err
	}
	return parseWith(ctx, proc, in, stripEmbeds)
}

// Close は保持するランタイムを解放する。
func (p *WASMParser) Close(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	var firstErr error
	for origin, proc := range p.processors {
		if err := proc.Close(ctx); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("text: failed to close processor for origin %q: %w", origin, err)
		}
		delete(p.processors, origin)
	}
	if err := p.runtime.Close(ctx); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("text: failed to close markdown runtime: %w", err)
	}
	return firstErr
}

// processorFor は origin に対応する Processor を返す。なければ作って保持する。
func (p *WASMParser) processorFor(
	ctx context.Context,
	origin string,
) (*markdown.Processor, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if proc, ok := p.processors[origin]; ok {
		return proc, nil
	}
	proc, err := p.runtime.NewProcessor(
		ctx,
		markdown.GrammarTraqV1,
		markdown.ProcessorOptions{Origin: origin},
	)
	if err != nil {
		return nil, fmt.Errorf("text: failed to create processor for origin %q: %w", origin, err)
	}
	p.processors[origin] = proc
	return proc, nil
}

// parseWith は Processor で2パス処理する。1回目の抽出結果 (ID) を保持し、
// 自分宛目印・埋め込み除去を施したソースで2回目を回して表示用本文を得る。
// 書き換えが不要なら2回目は省略する。
func parseWith(
	ctx context.Context,
	proc processor,
	in ParseInput,
	stripEmbeds bool,
) (Parsed, error) {
	first, err := proc.Process(ctx, in.Content)
	if err != nil {
		return Parsed{}, fmt.Errorf("text: failed to parse message: %w", err)
	}
	nodes, err := collectNodes(first.Document)
	if err != nil {
		return Parsed{}, err
	}
	rw := rewriter{
		selfID:      in.SelfID,
		groups:      normalizeGroups(in.Groups),
		stripEmbeds: stripEmbeds,
	}
	rewritten, selfMentioned, err := rw.rewrite(in.Content, nodes)
	if err != nil {
		return Parsed{}, err
	}
	body := first.Extraction.MessageText
	if rewritten != in.Content {
		second, err := proc.Process(ctx, rewritten)
		if err != nil {
			return Parsed{}, fmt.Errorf("text: failed to re-parse message: %w", err)
		}
		body = second.Extraction.MessageText
	}
	return Parsed{
		Body:          body,
		QuoteIDs:      append([]string(nil), first.Extraction.Citations...),
		FileIDs:       append([]string(nil), first.Extraction.Attachments...),
		SelfMentioned: selfMentioned,
	}, nil
}

// normalizeGroups はグループID照合用に小文字化した集合を作る。
func normalizeGroups(groups map[string]struct{}) map[string]struct{} {
	if len(groups) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(groups))
	for id := range groups {
		out[strings.ToLower(id)] = struct{}{}
	}
	return out
}

// sourceEdit はソースへの置換1件。
type sourceEdit struct {
	start, end uint32
	text       string
}

// applyEdits は後方から置換を適用する。span は重ならないことを前提とする。
func applyEdits(source string, edits []sourceEdit) string {
	sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	out := source
	for _, e := range edits {
		out = out[:e.start] + e.text + out[e.end:]
	}
	return out
}
