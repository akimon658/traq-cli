package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/akimon658/traq-cli/internal/foundation/config"
)

// runCLI は cobra コマンドを対話なしで実行し、標準出力を返す。
func runCLI(t *testing.T, e *config.Env, args ...string) (string, error) {
	t.Helper()
	cmd := newRootCmd(func() *config.Env { return e })
	cmd.SetArgs(args)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	execErr := cmd.Execute()
	return strings.TrimRight(stdout.String(), "\n"), execErr
}

// unitEnv は実サーバーを使わない単体テスト用の Env を返す。
func unitEnv() *config.Env {
	return &config.Env{Origin: "https://q.trap.jp", ClientID: "test"}
}
