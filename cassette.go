package mannequin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
)

// Cassette is a recorded set of request/response exchanges.
//
// Cassettes never store request headers, so credentials are structurally
// absent. Bodies are recorded verbatim: they can contain PII or
// proprietary prompts and outputs. Use WithScrub hooks or synthetic
// fixtures, and review cassettes before committing them.
type Cassette struct {
	Version   int        `json:"version"`
	Exchanges []Exchange `json:"exchanges"`
}

// Exchange is one recorded request/response pair.
type Exchange struct {
	Method       string          `json:"method"`
	Path         string          `json:"path"`
	RequestBody  json.RawMessage `json:"request_body,omitempty"`
	Status       int             `json:"status"`
	ResponseBody json.RawMessage `json:"response_body"`
}

type recorder struct {
	path  string
	scrub []func(*Exchange)

	mu        sync.Mutex
	exchanges []Exchange
	saved     bool
	err       error
}

// Record starts recording matched non-streaming exchanges to path.
// The cassette is saved by t.Cleanup (or Close). Streaming responses are
// not recorded. Headers are never recorded.
func (s *Server) Record(path string, opts ...func(*Exchange)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rec = &recorder{path: path, scrub: opts}
}

func (r *recorder) add(req *http.Request, reqBody []byte, resp response) {
	ex := Exchange{
		Method:       req.Method,
		Path:         req.URL.Path,
		Status:       resp.status,
		RequestBody:  json.RawMessage(normalizeJSON(reqBody)),
		ResponseBody: json.RawMessage(normalizeJSON(resp.body)),
	}
	for _, f := range r.scrub {
		f(&ex)
	}
	r.mu.Lock()
	r.exchanges = append(r.exchanges, ex)
	r.mu.Unlock()
}

func (r *recorder) save() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.saved {
		return
	}
	r.saved = true
	c := Cassette{Version: 1, Exchanges: r.exchanges}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		r.err = err
		return
	}
	r.err = os.WriteFile(r.path, data, 0o644)
}

// WithScrub adds a hook that can modify or censor each recorded exchange
// before it is stored. Returning an Exchange with nil bodies drops them.
func WithScrub(f func(*Exchange)) func(*Exchange) { return f }

// Replay answers requests from a recorded cassette. A request matches an
// exchange when the method and path suffix match and the JSON request
// bodies are semantically equal. Unmatched requests get a 422 error.
func Replay(path string) (Responder, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("mannequin: read cassette: %w", err)
	}
	var c Cassette
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("mannequin: parse cassette: %w", err)
	}
	return &replayResponder{c: c}, nil
}

// MustReplay is Replay that panics on error (handy in tests).
func MustReplay(path string) Responder {
	r, err := Replay(path)
	if err != nil {
		panic(err)
	}
	return r
}

type replayResponder struct {
	c Cassette
}

func (rr *replayResponder) respond(r *http.Request, reqBody []byte) response {
	for _, ex := range rr.c.Exchanges {
		if ex.Method != r.Method || !strings.HasSuffix(r.URL.Path, ex.Path) {
			continue
		}
		if !jsonEqual(ex.RequestBody, reqBody) {
			continue
		}
		return response{status: ex.Status, header: jsonHeader(), body: []byte(ex.ResponseBody)}
	}
	return response{
		status: 422,
		header: jsonHeader(),
		body: mustJSON(map[string]any{
			"error": map[string]any{
				"type":    "mannequin_cassette_miss",
				"message": "mannequin: no cassette exchange matched " + r.Method + " " + r.URL.Path,
			},
		}),
	}
}

// normalizeJSON returns canonical JSON for b, or a JSON string wrapping
// non-JSON input. Empty input becomes null.
func normalizeJSON(b []byte) []byte {
	if len(bytes.TrimSpace(b)) == 0 {
		return []byte("null")
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return mustJSON(string(b))
	}
	return mustJSON(v)
}

// jsonEqual reports semantic equality of two JSON documents.
func jsonEqual(a json.RawMessage, b []byte) bool {
	var va, vb any
	if err := json.Unmarshal(a, &va); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &vb); err != nil {
		return false
	}
	return bytes.Equal(mustJSON(va), mustJSON(vb))
}
