package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"math/big"
	"net/url"
	"strings"
)

const pkceAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~"

const (
	pkceVerifierLength = 64
	pkceStateLength    = 32
)

// authorizePath は認可エンドポイントのパス。PKCE の認可 URL 組み立てに使う。
const authorizePath = "/api/v3/oauth2/authorize"

// PKCE は OAuth2 authorization code flow (RFC 8252) の検証子・チャレンジ・状態を保持する。
type PKCE struct {
	Verifier  string
	Challenge string
	State     string
}

// NewPKCE はセキュア乱数で PKCE 検証子と状態を生成する。
func NewPKCE() (PKCE, error) {
	verifier, err := randString(pkceVerifierLength)
	if err != nil {
		return PKCE{}, err
	}
	state, err := randString(pkceStateLength)
	if err != nil {
		return PKCE{}, err
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	return PKCE{
		Verifier:  verifier,
		Challenge: challenge,
		State:     state,
	}, nil
}

// AuthorizeURL は認可 URL を組み立てる。
func (pk PKCE) AuthorizeURL(origin, clientID, redirectURI string, scopes []string) string {
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"scope":                 {strings.Join(scopes, " ")},
		"state":                 {pk.State},
		"code_challenge":        {pk.Challenge},
		"code_challenge_method": {"S256"},
	}
	return origin + authorizePath + "?" + q.Encode()
}

func randString(n int) (string, error) {
	out := make([]byte, n)
	max := big.NewInt(int64(len(pkceAlphabet)))
	for i := range out {
		r, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		out[i] = pkceAlphabet[r.Int64()]
	}
	return string(out), nil
}
