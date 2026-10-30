package cli

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/moznion/go-optional"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/compose"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/authflow"
	"github.com/akimon658/traq-cli/internal/client"
	"github.com/akimon658/traq-cli/internal/config"
	"github.com/akimon658/traq-cli/internal/exit"
)

const testBotName = "traq-cli"

var (
	testTraqOrigin string
	testTraqToken  string
	traqStackReady bool
)

func TestMain(m *testing.M) {
	// testing.Short() を使うには flag をパースしておく必要がある
	flag.Parse()
	if testing.Short() {
		os.Exit(m.Run())
	}

	// 詳細な理由は不明だが Ryuk が有効だと不安定になることがあるため無効化する
	// クリーンアップは明示的に composeStack.Down を呼ぶため Ryuk 無しでも問題ない
	_ = os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")

	// OAuth ブラウザログインのタイムアウトはテストで hang を避けるため短縮する
	authflow.LoginTimeout = 30 * time.Second

	ctx := context.Background()
	stack, err := compose.NewDockerComposeWith(compose.WithStackFiles("../../compose.yaml"))
	if err != nil {
		panic(fmt.Sprintf("failed to create compose stack: %v", err))
	}

	if err := stack.
		WithEnv(map[string]string{"TRAQ_SERVER_PORT": "0"}).
		WaitForService("mariadb", wait.ForHealthCheck().WithStartupTimeout(time.Minute)).
		WaitForService("es", wait.ForHealthCheck().WithStartupTimeout(3*time.Minute)).
		WaitForService(
			"traq_server",
			wait.ForHTTP("/api/v3/version").WithPort("3000/tcp").WithStartupTimeout(3*time.Minute),
		).
		Up(ctx, compose.Wait(true), compose.RunServices("mariadb", "es", "traq_server")); err != nil {
		panic(fmt.Sprintf("failed to start compose stack: %v", err))
	}

	c, err := stack.ServiceContainer(ctx, "traq_server")
	if err != nil {
		panic(fmt.Sprintf("failed to get traQ server container: %v", err))
	}
	host, err := c.Host(ctx)
	if err != nil {
		panic(fmt.Sprintf("failed to get host: %v", err))
	}
	port, err := c.MappedPort(ctx, "3000/tcp")
	if err != nil {
		panic(fmt.Sprintf("failed to get mapped port: %v", err))
	}
	testTraqOrigin = fmt.Sprintf("http://%s:%s", host, port.Port())
	token, err := createBotToken(testTraqOrigin)
	if err != nil {
		panic(fmt.Sprintf("failed to create a bot in %s: %v", testTraqOrigin, err))
	}
	testTraqToken = token
	traqStackReady = true

	code := m.Run()
	if err := stack.Down(
		ctx,
		compose.RemoveOrphans(true),
		compose.RemoveImagesLocal,
		compose.RemoveVolumes(true),
	); err != nil {
		fmt.Fprintf(os.Stderr, "failed to stop compose stack: %v\n", err)
		if code == 0 {
			code = exit.General
		}
	}
	os.Exit(code)
}

// traqClient は traqOrigin の traQ API を叩く go-traq クライアントを返す。
// sess が Some の場合はセッションクッキーを全リクエストに付与する。
func traqClient(traqOrigin string, sess optional.Option[*http.Cookie]) *traq.APIClient {
	cfg := traq.NewConfiguration()
	cfg.Servers = traq.ServerConfigurations{{URL: traqOrigin + "/api/v3"}}
	sess.IfSome(func(ck *http.Cookie) {
		cfg.DefaultHeader["Cookie"] = ck.Name + "=" + ck.Value
	})
	return traq.NewAPIClient(cfg)
}

// adminSession は traQ に管理者 (traq/traq) でログインし、セッションクッキーを返す。
func adminSession(traqOrigin string) (*http.Cookie, error) {
	resp, err := traqClient(traqOrigin, optional.None[*http.Cookie]()).
		AuthenticationAPI.Login(context.Background()).
		PostLoginRequest(traq.PostLoginRequest{Name: "traq", Password: "traq"}).
		Execute()
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	for _, ck := range resp.Cookies() {
		if ck.Name == "r_session" {
			return ck, nil
		}
	}
	return nil, errors.New("r_session cookie not found in login response")
}

// createBotToken は traQ に管理者 (traq/traq) でログインし、テスト用 Bot を作成して
// そのアクセストークンを返す。
func createBotToken(traqOrigin string) (string, error) {
	session, err := adminSession(traqOrigin)
	if err != nil {
		return "", err
	}
	bot, _, err := traqClient(traqOrigin, optional.Some(session)).
		BotAPI.CreateBot(context.Background()).
		PostBotRequest(traq.PostBotRequest{
			Name:        testBotName,
			DisplayName: "traQ CLI",
			Description: "traQ CLI integration test bot",
			Mode:        traq.BOTMODE_HTTP,
			Endpoint:    traq.PtrString("http://localhost"),
		}).
		Execute()
	if err != nil {
		return "", fmt.Errorf("failed to create a bot: %w", err)
	}
	if bot.Tokens.AccessToken == "" {
		return "", errors.New("no access token in the response")
	}
	return bot.Tokens.AccessToken, nil
}

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

// botEnv は bot トークンを環境変数経由で渡すのと等価な Env を返す。
func botEnv(t *testing.T, token string) *config.Env {
	t.Helper()
	return &config.Env{Origin: testTraqOrigin, AccessToken: token}
}

// botAPIClient はテストから traQ API を直接叩くための go-traq クライアントを返す。
func botAPIClient(t *testing.T) *traq.APIClient {
	t.Helper()
	return traqAPIClient(t, optional.None[*http.Cookie]())
}

func getMe(t *testing.T, token string) *traq.MyUserDetail {
	t.Helper()
	me, _, err := botAPIClient(t).MeAPI.GetMe(
		client.TokenContext(t.Context(), token),
	).Execute()
	require.NoError(t, err, "failed to get the current user")
	return me
}

func getMessage(t *testing.T, messageID string) *traq.Message {
	t.Helper()
	msg, _, err := botAPIClient(t).MessageAPI.GetMessage(
		client.TokenContext(t.Context(), testTraqToken), messageID,
	).Execute()
	require.NoError(t, err, "failed to get message %s", messageID)
	return msg
}

func getChannels(t *testing.T) *traq.ChannelList {
	t.Helper()
	list, _, err := botAPIClient(t).ChannelAPI.GetChannels(
		client.TokenContext(t.Context(), testTraqToken),
	).Execute()
	require.NoError(t, err, "failed to get channels")
	return list
}

func overrideEnv(t *testing.T, ms *auth.MockStore, clientIDs ...string) *config.Env {
	t.Helper()
	clientID := "test"
	if len(clientIDs) > 0 && clientIDs[0] != "" {
		clientID = clientIDs[0]
	}
	return &config.Env{Origin: testTraqOrigin, ClientID: clientID, Store: ms}
}

func requireTraqStack(t *testing.T) {
	t.Helper()
	if !traqStackReady {
		t.Skip("traQ test stack is not running (try without -short)")
	}
}
