package cli

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	gotraq "github.com/traPtitech/go-traq"
	"go.uber.org/mock/gomock"

	"github.com/akimon658/traq-cli/internal/foundation/config"
	"github.com/akimon658/traq-cli/internal/traq"
)

// runCLI は cobra コマンドを対話なしで実行し、標準出力を返す。
func runCLI(t *testing.T, e *config.Env, args ...string) (string, error) {
	t.Helper()
	return runCLIWithResolver(t, e, nil, args...)
}

// runCLIWithResolver は認証済みクライアントの解決子を差し替えて実行する。
// resolve に nil を渡すと本番の defaultResolveClient を使う。
// 各コマンドツリーが独自のクロージャを持つため t.Parallel と併用できる。
func runCLIWithResolver(
	t *testing.T,
	e *config.Env,
	resolve resolveFunc,
	args ...string,
) (string, error) {
	t.Helper()
	cmd := newRootCmd(func() *config.Env { return e }, resolve)
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

// stubEnrichment は view の遅延解決の happy path をモックする。
// 出力内容は view 側で検証するため、ここでは呼び出しの成功だけを保証する。
func stubEnrichment(mc *traq.MockClient) {
	mc.EXPECT().GetSelfIDAndGroups(gomock.Any()).Return("", map[string]struct{}{}, nil).AnyTimes()
	mc.EXPECT().GetUserByID(gomock.Any(), gomock.Any()).
		Return(gotraq.User{}, errors.New("not found")).AnyTimes()
	mc.EXPECT().GetStampName(gomock.Any(), gomock.Any()).
		Return("", errors.New("not found")).AnyTimes()
	mc.EXPECT().Origin().Return("https://q.trap.jp").AnyTimes()
}

func stubResolve(mc traq.Client, err error) resolveFunc {
	return func(_ *cobra.Command, _ *config.Env) (traq.Client, error) {
		if err != nil {
			return nil, err
		}
		return mc, nil
	}
}

func writeTempDownload(t *testing.T, content string) *os.File {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "traq-cli-download-*")
	require.NoError(t, err)
	_, err = f.WriteString(content)
	require.NoError(t, err)
	_, err = f.Seek(0, 0)
	require.NoError(t, err)
	return f
}
