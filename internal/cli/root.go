package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/akimon658/traq-cli/internal/config"
	"github.com/akimon658/traq-cli/internal/exit"
)

// globalFlags は root の永続フラグの値を保持する。
// コマンドツリーごとに独立させるため、パッケージ変数ではなくローカルに持つ。
type globalFlags struct {
	token    string
	origin   string
	clientID string
}

// apply は明示的に指定されたフラグだけを e に反映する。
func (f *globalFlags) apply(cmd *cobra.Command, e *config.Env) {
	if cmd.Flags().Changed("origin") {
		e.Origin = f.origin
	}
	if cmd.Flags().Changed("client-id") {
		e.ClientID = f.clientID
	}
	if cmd.Flags().Changed("token") {
		e.TokenOverride = f.token
	}
}

// newRootCmd はコマンドツリーを新しく構築して返す。newEnv は各コマンド実行時に
// 呼ばれ、テストから Env を注入できるようにするためのもの。
func newRootCmd(newEnv func() *config.Env) *cobra.Command {
	flags := &globalFlags{}
	rootCmd := &cobra.Command{
		Use:   "traq",
		Short: "traQ (https://github.com/traPtitech/traQ) を操作する CLI ツール",
		CompletionOptions: cobra.CompletionOptions{
			HiddenDefaultCmd: true,
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	rootCmd.PersistentFlags().
		StringVar(&flags.token, "token", "", "使用するトークン (保存済みトークンを一時的に上書き; 省略時は環境変数 TRAQ_ACCESS_TOKEN)")
	rootCmd.PersistentFlags().
		StringVar(&flags.origin, "origin", "", "traQ サーバーの origin (省略時は環境変数 TRAQ_ORIGIN)")
	rootCmd.PersistentFlags().
		StringVar(&flags.clientID, "client-id", "", "OAuth2 クライアント ID (省略時は環境変数 TRAQ_CLIENT_ID)")
	rootCmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return exit.UsageError(err)
	})
	rootCmd.AddCommand(
		newAuthCmd(newEnv, flags),
		newUserCmd(newEnv, flags),
		newChannelCmd(newEnv, flags),
		newMessageCmd(newEnv, flags),
	)
	return rootCmd
}

// Execute は CLI を実行し、終了コードを返す。
func Execute() int {
	err := newRootCmd(config.NewEnv).Execute()
	if err == nil {
		return 0
	}
	var ec *exit.Error
	if errors.As(err, &ec) {
		fmt.Fprintln(os.Stderr, ec.Error())
		return ec.Code
	}
	if strings.HasPrefix(err.Error(), "unknown command") {
		fmt.Fprintln(os.Stderr, err)
		return exit.Usage
	}
	fmt.Fprintln(os.Stderr, err)
	return exit.General
}
