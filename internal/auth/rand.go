package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"math/big"
)

const pkceAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~"

const (
	pkceVerifierLength = 64
	pkceStateLength    = 32
)

type PKCE struct {
	Verifier  string
	Challenge string
	State     string
}

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
