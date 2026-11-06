package traq

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// callbackPath はループバックコールバックサーバーのパス。
const callbackPath = "/callback"

type CallbackServer struct {
	srv    *http.Server
	l      net.Listener
	state  string
	result chan callbackResult
}

type callbackResult struct {
	code string
	err  error
}

const callbackReadHeaderTimeout = 10 * time.Second

func ListenCallback(state string) (*CallbackServer, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to start a local callback server: %w", err)
	}
	cs := &CallbackServer{
		srv:    &http.Server{ReadHeaderTimeout: callbackReadHeaderTimeout},
		l:      l,
		state:  state,
		result: make(chan callbackResult, 1),
	}
	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, cs.handle)
	cs.srv.Handler = mux
	go func() {
		_ = cs.srv.Serve(l)
	}()
	return cs, nil
}

func (s *CallbackServer) RedirectURI() string {
	return fmt.Sprintf("http://127.0.0.1:%d%s", s.l.Addr().(*net.TCPAddr).Port, callbackPath)
}

func (s *CallbackServer) Wait(ctx context.Context) (string, error) {
	select {
	case res := <-s.result:
		if res.err != nil {
			return "", res.err
		}
		return res.code, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (s *CallbackServer) Close() {
	_ = s.srv.Close()
}

func (s *CallbackServer) handle(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	res := callbackResult{}
	switch {
	case q.Get("error") != "":
		res.err = fmt.Errorf("authorization was denied (%s)", q.Get("error"))
		if d := q.Get("error_description"); d != "" {
			res.err = fmt.Errorf("authorization was denied (%s: %s)", q.Get("error"), d)
		}
	case q.Get("code") == "":
		res.err = errors.New("authorization callback has no code")
	case q.Get("state") != s.state:
		res.err = errors.New("authorization callback state mismatch")
	default:
		res.code = q.Get("code")
	}
	if res.err != nil {
		http.Error(w, "Authorization failed: "+res.err.Error(), http.StatusBadRequest)
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprint(w, "Authorization complete. You can close this tab.")
	}
	select {
	case s.result <- res:
	default:
	}
}
