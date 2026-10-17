package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/compose"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/akimon658/traq-cli/internal/auth"
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
	loginTimeout = 30 * time.Second

	ctx := context.Background()
	stack, err := compose.NewDockerComposeWith(compose.WithStackFiles("../../compose.yaml"))
	if err != nil {
		panic(fmt.Sprintf("failed to create compose stack: %v", err))
	}

	if err := stack.
		WithEnv(map[string]string{"TRAQ_SERVER_PORT": "0"}).
		WaitForService("mariadb", wait.ForHealthCheck().WithStartupTimeout(time.Minute)).
		WaitForService(
			"traq_server",
			wait.ForHTTP("/api/v3/version").WithPort("3000/tcp").WithStartupTimeout(2*time.Minute),
		).
		Up(ctx, compose.Wait(true), compose.RunServices("mariadb", "traq_server")); err != nil {
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
			code = exitGeneral
		}
	}
	os.Exit(code)
}

// adminSession は traQ に管理者 (traq/traq) でログインし、セッションクッキーを返す。
func adminSession(traqOrigin string) (*http.Cookie, error) {
	apiBase := traqOrigin + "/api/v3"

	loginBody, err := json.Marshal(map[string]string{"name": "traq", "password": "traq"})
	if err != nil {
		return nil, err
	}
	loginResp, err := http.Post(apiBase+"/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		return nil, err
	}
	defer func() { _ = loginResp.Body.Close() }()
	body, err := io.ReadAll(loginResp.Body)
	if err != nil {
		return nil, err
	}
	if loginResp.StatusCode != http.StatusNoContent {
		return nil, fmt.Errorf(
			"login failed: status %d, body: %s", loginResp.StatusCode, string(body),
		)
	}
	for _, ck := range loginResp.Cookies() {
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
	apiBase := traqOrigin + "/api/v3"

	botBody, err := json.Marshal(map[string]string{
		"name":        testBotName,
		"displayName": "traQ CLI",
		"description": "traQ CLI integration test bot",
		"mode":        "HTTP",
		"endpoint":    "http://localhost",
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, apiBase+"/bots", bytes.NewReader(botBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(session)
	createResp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = createResp.Body.Close() }()
	body, err := io.ReadAll(createResp.Body)
	if err != nil {
		return "", err
	}
	if createResp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf(
			"failed to create a bot: status %d, body: %s", createResp.StatusCode, string(body),
		)
	}
	var created struct {
		Tokens struct {
			AccessToken string `json:"accessToken"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		return "", err
	}
	if created.Tokens.AccessToken == "" {
		return "", fmt.Errorf("no access token in the response: %s", string(body))
	}
	return created.Tokens.AccessToken, nil
}

func runCLI(t *testing.T, e *Env, args ...string) (string, error) {
	t.Helper()
	cmd := newRootCmd(func() *Env { return e })
	cmd.SetArgs(args)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	execErr := cmd.Execute()
	return strings.TrimRight(stdout.String(), "\n"), execErr
}

// botEnv は bot トークンを環境変数経由で渡すのと等価な Env を返す。
func botEnv(t *testing.T, token string) *Env {
	t.Helper()
	return &Env{Origin: testTraqOrigin, AccessToken: token}
}

func apiGetJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	apiMe, err := jsonGet(t, testTraqOrigin+"/api/v3"+path)
	require.NoError(t, err, "failed to GET %s", path)
	return apiMe
}

func jsonGet(t *testing.T, u string) (map[string]any, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+testTraqToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}
	var v map[string]any
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, err
	}
	return v, nil
}

func overrideEnv(t *testing.T, ms *auth.MockStore, clientIDs ...string) *Env {
	t.Helper()
	clientID := "test"
	if len(clientIDs) > 0 && clientIDs[0] != "" {
		clientID = clientIDs[0]
	}
	return &Env{Origin: testTraqOrigin, ClientID: clientID, Store: ms}
}

func requireTraqStack(t *testing.T) {
	t.Helper()
	if !traqStackReady {
		t.Skip("traQ test stack is not running (try without -short)")
	}
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}
