package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const (
	exitGeneral           = 1
	exitUsage             = 2
	exitAuthRequired      = 3
	exitScopeInsufficient = 4
)

type ExitCodeError struct {
	Code int
	Err  error
}

func (e *ExitCodeError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("exit status %d", e.Code)
}

func (e *ExitCodeError) Unwrap() error {
	return e.Err
}

var (
	flagToken    string
	flagOrigin   string
	flagClientID string
)

var rootCmd = &cobra.Command{
	Use:   "traq",
	Short: "traQ (https://github.com/traPtitech/traQ) を操作する CLI ツール",
	CompletionOptions: cobra.CompletionOptions{
		HiddenDefaultCmd: true,
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().
		StringVar(&flagToken, "token", "", "使用するトークン (保存済みトークンを一時的に上書き; 省略時は環境変数 TRAQ_ACCESS_TOKEN)")
	rootCmd.PersistentFlags().
		StringVar(&flagOrigin, "origin", "", "traQ サーバーの origin (省略時は環境変数 TRAQ_ORIGIN)")
	rootCmd.PersistentFlags().
		StringVar(&flagClientID, "client-id", "", "OAuth2 クライアント ID (省略時は環境変数 TRAQ_CLIENT_ID)")
	rootCmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &ExitCodeError{exitUsage, err}
	})
	rootCmd.AddCommand(authCmd, usersCmd, messageCmd)
}

func Execute() int {
	err := rootCmd.Execute()
	if err == nil {
		return 0
	}
	var ec *ExitCodeError
	if errors.As(err, &ec) {
		fmt.Fprintln(os.Stderr, ec.Error())
		return ec.Code
	}
	if strings.HasPrefix(err.Error(), "unknown command") {
		fmt.Fprintln(os.Stderr, err)
		return exitUsage
	}
	fmt.Fprintln(os.Stderr, err)
	return exitGeneral
}
