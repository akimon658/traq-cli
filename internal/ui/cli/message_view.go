package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/akimon658/traq-cli/internal/domain/traq"
	"github.com/akimon658/traq-cli/internal/foundation/text"
	"github.com/akimon658/traq-cli/internal/ui/view"
)

// newMessageRenderer はメッセージ表示器を作る。パーサ生成の失敗はそのまま返す。
func newMessageRenderer(
	ctx context.Context,
	api traq.Client,
) (*view.MessageRenderer, error) {
	parser, err := text.NewWASMParser(ctx)
	if err != nil {
		return nil, err
	}
	return view.NewMessageRenderer(api, time.Local, parser), nil
}

// closeRenderer は表示器を解放する。失敗時は警告に倒す。
func closeRenderer(cmd *cobra.Command, r *view.MessageRenderer) {
	if err := r.Close(cmd.Context()); err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), "warning: failed to close message parser:", err)
	}
}
