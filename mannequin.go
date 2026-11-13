// Package mannequin provides a stand-in LLM server for Go tests.
//
// Point your OpenAI or Anthropic SDK at a mannequin server and program
// deterministic responses: fixed text, streamed chunks, failure modes,
// or recorded cassettes. No live API key is needed in CI.
package mannequin

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// Server is a stand-in LLM HTTP server.
type Server struct {
	srv *httptest.Server

	mu    sync.Mutex
	rules []rule
	rec   *recorder
}

// New starts a stand-in server. If t is non-nil the server (and any
// pending cassette save) is handled by t.Cleanup.
func New(t testing.TB) *Server {
	s := &Server{}
	s.srv = httptest.NewServer(http.HandlerFunc(s.handle))
	if t != nil {
		t.Cleanup(func() {
			if s.rec != nil {
				s.rec.save()
			}
			s.srv.Close()
		})
	}
	return s
}

// URL is the base URL to hand to your SDK (e.g. option.WithBaseURL).
func (s *Server) URL() string { return s.srv.URL }

// Close shuts the server down and saves any pending cassette.
// Only needed when t was nil in New.
func (s *Server) Close() {
	if s.rec != nil {
		s.rec.save()
	}
	s.srv.Close()
}

// Matcher reports whether a request should be answered by a rule.
type Matcher func(r *http.Request, body []byte) bool

// Chat matches OpenAI POST /v1/chat/completions requests.
func Chat() Matcher {
	return func(r *http.Request, _ []byte) bool {
		return r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/chat/completions")
	}
}

// Messages matches Anthropic POST /v1/messages requests.
func Messages() Matcher {
	return func(r *http.Request, _ []byte) bool {
		return r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/messages")
	}
}

// Path matches an exact method and path suffix.
func Path(method, suffix string) Matcher {
	return func(r *http.Request, _ []byte) bool {
		return r.Method == method && strings.HasSuffix(r.URL.Path, suffix)
	}
}

// BodyContains restricts a matcher to requests whose body contains sub
// (e.g. a model name or prompt fragment).
func BodyContains(m Matcher, sub string) Matcher {
	return func(r *http.Request, body []byte) bool {
		return m(r, body) && strings.Contains(string(body), sub)
	}
}

// Responder produces a programmed response.
type Responder interface {
	respond(r *http.Request, reqBody []byte) response
}

type rule struct {
	match   Matcher
	respond Responder
}

// On registers a rule. Rules are tried in registration order; the first
// matching rule answers. Unmatched requests get a 404 JSON error.
func (s *Server) On(m Matcher, r Responder) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rules = append(s.rules, rule{m, r})
}

// Func adapts a plain function into a Responder for full control.
// The returned response struct is package-private; use Text, Stream,
// Fail or Replay to build responses, or write raw with Raw.
func Func(f func(r *http.Request, reqBody []byte) (status int, body []byte)) Responder {
	return funcResponder(f)
}

type funcResponder func(r *http.Request, reqBody []byte) (int, []byte)

func (f funcResponder) respond(r *http.Request, reqBody []byte) response {
	status, body := f(r, reqBody)
	return response{status: status, header: jsonHeader(), body: body}
}

func jsonHeader() http.Header {
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	return h
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	_ = r.Body.Close()

	s.mu.Lock()
	rules := make([]rule, len(s.rules))
	copy(rules, s.rules)
	rec := s.rec
	s.mu.Unlock()

	for _, rl := range rules {
		if rl.match(r, body) {
			resp := rl.respond.respond(r, body)
			resp.write(w)
			if rec != nil && resp.stream == nil {
				rec.add(r, body, resp)
			}
			return
		}
	}
	resp := response{
		status: http.StatusNotFound,
		header: jsonHeader(),
		body: mustJSON(map[string]any{
			"error": map[string]any{
				"type":    "mannequin_unmatched_request",
				"message": "mannequin: no rule matched " + r.Method + " " + r.URL.Path,
			},
		}),
	}
	resp.write(w)
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
