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

	"github.com/stretchr/testify/assert"
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

// createBotToken は traQ に管理者 (traq/traq) でログインし、テスト用 Bot を作成して
// そのアクセストークンを返す。
func createBotToken(traqOrigin string) (string, error) {
	apiBase := traqOrigin + "/api/v3"

	loginBody, err := json.Marshal(map[string]string{"name": "traq", "password": "traq"})
	if err != nil {
		return "", err
	}
	loginResp, err := http.Post(apiBase+"/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		return "", err
	}
	defer func() { _ = loginResp.Body.Close() }()
	body, err := io.ReadAll(loginResp.Body)
	if err != nil {
		return "", err
	}
	if loginResp.StatusCode != http.StatusNoContent {
		return "", fmt.Errorf(
			"login failed: status %d, body: %s", loginResp.StatusCode, string(body),
		)
	}
	var session *http.Cookie
	for _, ck := range loginResp.Cookies() {
		if ck.Name == "r_session" {
			session = ck
			break
		}
	}
	if session == nil {
		return "", errors.New("r_session cookie not found in login response")
	}

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
	body, err = io.ReadAll(createResp.Body)
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

func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	flagToken, flagOrigin, flagClientID = "", "", ""
	rootCmd.SetArgs(args)
	out, execErr := captureStdout(t, rootCmd.Execute)
	return strings.TrimRight(out, "\n"), execErr
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	stdout := os.Stdout
	os.Stdout = w
	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		defer close(done)
		_, _ = buf.ReadFrom(r)
	}()
	execErr := fn()
	os.Stdout = stdout
	require.NoError(t, w.Close())
	<-done
	require.NoError(t, r.Close())
	return buf.String(), execErr
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

func TestUsersMe(t *testing.T) {
	requireTraqStack(t)
	t.Setenv("TRAQ_ORIGIN", testTraqOrigin)
	t.Setenv("TRAQ_ACCESS_TOKEN", testTraqToken)

	out, err := runCLI(t, "users", "me")
	require.NoError(t, err, "users me failed")
	var me map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &me), "output is not JSON: %s", out)
	apiMe := apiGetJSON(t, "/users/me")
	assert.Equal(t, apiMe["id"], me["id"])
	assert.Contains(t, strings.ToLower(asString(me["name"])), testBotName)
}

func TestUsersMeInvalidToken(t *testing.T) {
	requireTraqStack(t)
	t.Setenv("TRAQ_ORIGIN", testTraqOrigin)
	t.Setenv("TRAQ_ACCESS_TOKEN", "invalid")

	_, err := runCLI(t, "users", "me")
	require.Error(t, err, "expected an error for the invalid token")
	var ec *ExitCodeError
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exitAuthRequired, ec.Code)
	assert.Contains(t, err.Error(), "the token was rejected")
}

func TestMessageSend(t *testing.T) {
	requireTraqStack(t)
	t.Setenv("TRAQ_ORIGIN", testTraqOrigin)
	t.Setenv("TRAQ_ACCESS_TOKEN", testTraqToken)

	const (
		contentByPath = "hello traq-cli (by #general path)"
		contentByUUID = "hello traq-cli (by channel UUID)"
	)

	out, err := runCLI(t, "message", "send", "#general", contentByPath)
	require.NoError(t, err, "message send failed")
	assertPosted(t, out, contentByPath)

	channelID := testGeneralChannelID(t)
	out, err = runCLI(t, "message", "send", channelID, contentByUUID)
	require.NoError(t, err, "message send failed")
	assertPosted(t, out, contentByUUID)
}

func TestMessageSendChannelNotFound(t *testing.T) {
	requireTraqStack(t)
	t.Setenv("TRAQ_ORIGIN", testTraqOrigin)
	t.Setenv("TRAQ_ACCESS_TOKEN", testTraqToken)

	out, err := runCLI(t, "message", "send", "#does-not-exist", "hello")
	require.Error(t, err, "expected an error for the nonexistent channel, output: %s", out)
	var ec *ExitCodeError
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exitGeneral, ec.Code)
	assert.Contains(t, err.Error(), "channel not found")
}

func assertPosted(t *testing.T, out, wantContent string) {
	t.Helper()
	var msg map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &msg), "output is not JSON: %s", out)
	id, ok := msg["id"].(string)
	require.True(t, ok && id != "", "no message id in the output: %s", out)
	got := apiGetJSON(t, "/messages/"+id)
	assert.Equal(t, wantContent, asString(got["content"]))
}

func testGeneralChannelID(t *testing.T) string {
	t.Helper()
	list := apiGetJSON(t, "/channels")
	pub, ok := list["public"].([]any)
	require.True(t, ok, "unexpected channels response: %v", list)
	for _, ch := range pub {
		m, ok := ch.(map[string]any)
		if !ok {
			continue
		}
		name, id := asString(m["name"]), asString(m["id"])
		if name == "general" && id != "" {
			return id
		}
	}
	require.Fail(t, "channel general not found in the response", "%v", list)
	return ""
}

func TestAuthStatus(t *testing.T) {
	requireTraqStack(t)
	t.Setenv("TRAQ_ORIGIN", testTraqOrigin)
	t.Setenv("TRAQ_ACCESS_TOKEN", testTraqToken)

	out, err := runCLI(t, "auth", "status")
	require.NoError(t, err, "auth status failed")
	var status map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &status), "output is not JSON: %s", out)
	me := apiGetJSON(t, "/users/me")
	for key, want := range map[string]any{
		"origin": testTraqOrigin,
		"source": "environment",
		"method": "bot",
		"userId": asString(me["id"]),
		"user":   "@" + asString(me["name"]),
	} {
		assert.Equal(t, want, status[key], "status[%q]", key)
	}
}

func TestAuthLoginTokenAndLogout(t *testing.T) {
	requireTraqStack(t)
	fs := &fakeStore{}
	overrideEnv(t, fs)

	out, err := runCLI(t, "auth", "login", "--token", testTraqToken)
	require.NoError(t, err, "auth login failed")
	assert.Contains(t, out, "signed in: @")
	assert.Equal(t, 1, fs.saves, "store.Save called %d times, want 1", fs.saves)
	tok := fs.token
	assert.Equal(t, testTraqToken, tok.AccessToken)
	assert.Equal(t, auth.MethodBot, tok.Method)
	assert.NotEmpty(t, tok.UserName, "userName is empty")
	assert.Equal(t, testTraqOrigin, tok.Origin)

	fs.found = true

	out, err = runCLI(t, "auth", "logout")
	require.NoError(t, err, "auth logout failed")
	assert.Contains(t, out, "logged out")
	assert.Equal(t, 1, fs.deletes, "store.Delete called %d times, want 1", fs.deletes)

	_, err = runCLI(t, "auth", "logout")
	var ec *ExitCodeError
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exitAuthRequired, ec.Code)
}

func TestAuthLoginInvalidToken(t *testing.T) {
	requireTraqStack(t)
	fs := &fakeStore{}
	overrideEnv(t, fs)

	_, err := runCLI(t, "auth", "login", "--token", "invalid")
	require.Error(t, err, "expected an error for the invalid token")
	var ec *ExitCodeError
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exitGeneral, ec.Code)
	assert.Contains(t, err.Error(), "the token was rejected")
	assert.Zero(t, fs.saves, "store.Save called %d times, want 0", fs.saves)
}

func overrideEnv(t *testing.T, fs *fakeStore) {
	t.Helper()
	old := newEnv
	newEnv = func() *Env { return &Env{Origin: testTraqOrigin, ClientID: "test", Store: fs} }
	t.Cleanup(func() { newEnv = old })
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
