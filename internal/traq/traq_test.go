package traq

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/moznion/go-optional"
	"github.com/testcontainers/testcontainers-go/modules/compose"
	"github.com/testcontainers/testcontainers-go/wait"
	gotraq "github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/foundation/exit"
)

const testBotName = "traq-cli"

var (
	testTraqOrigin string
	testBotToken   string
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
	LoginTimeout = 30 * time.Second

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
	testBotToken = token
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

func requireTraqStack(t *testing.T) {
	t.Helper()
	if !traqStackReady {
		t.Skip("traQ test stack is not running (try without -short)")
	}
}

// testBotClient は Bot トークンに束縛された Client を返す。
func testBotClient(t *testing.T) Client {
	t.Helper()
	return NewService(testTraqOrigin, "test", nil, nil).
		WithToken(&ResolvedToken{Token: &Token{AccessToken: testBotToken}})
}

// traqClient は traqOrigin の traQ API を叩く go-traq クライアントを返す。
// sess が Some の場合はセッションクッキーを全リクエストに付与する。
func traqClient(traqOrigin string, sess optional.Option[*http.Cookie]) *gotraq.APIClient {
	cfg := gotraq.NewConfiguration()
	cfg.Servers = gotraq.ServerConfigurations{{URL: traqOrigin + "/api/v3"}}
	sess.IfSome(func(ck *http.Cookie) {
		cfg.DefaultHeader["Cookie"] = ck.Name + "=" + ck.Value
	})
	return gotraq.NewAPIClient(cfg)
}

// adminSession は traQ に管理者 (traq/traq) でログインし、セッションクッキーを返す。
func adminSession(traqOrigin string) (*http.Cookie, error) {
	resp, err := traqClient(traqOrigin, optional.None[*http.Cookie]()).
		AuthenticationAPI.Login(context.Background()).
		PostLoginRequest(gotraq.PostLoginRequest{Name: "traq", Password: "traq"}).
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
		PostBotRequest(gotraq.PostBotRequest{
			Name:        testBotName,
			DisplayName: "traQ CLI",
			Description: "traQ CLI integration test bot",
			Mode:        gotraq.BOTMODE_HTTP,
			Endpoint:    gotraq.PtrString("http://localhost"),
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

// testGeneralChannelID は #general のチャンネル ID を返す。
func testGeneralChannelID(t *testing.T) string {
	t.Helper()
	list, _, err := traqClient(testTraqOrigin, optional.None[*http.Cookie]()).
		ChannelAPI.GetChannels(TokenContext(t.Context(), testBotToken)).
		Execute()
	if err != nil {
		t.Fatalf("failed to get channels: %v", err)
	}
	for _, ch := range list.Public {
		if ch.Name == "general" && ch.Id != "" {
			return ch.Id
		}
	}
	t.Fatal("channel general not found in the response")
	return ""
}
