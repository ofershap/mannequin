package mannequin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// response is one programmed answer. Exactly one of body or stream is set.
type response struct {
	status int
	header http.Header
	body   []byte
	stream *streamSpec
}

// streamSpec describes an SSE stream to emit. When cut is true the
// stream stops after the given chunks: no terminating event, no clean
// end of body - the connection dies mid-stream, like a provider dropping.
type streamSpec struct {
	format streamFormat
	chunks []string
	cut    bool
}

type streamFormat int

const (
	formatOpenAI streamFormat = iota
	formatAnthropic
)

func (resp response) write(w http.ResponseWriter) {
	for k, vs := range resp.header {
		for _, v := range vs {
			w.Header().Set(k, v)
		}
	}
	if resp.stream == nil {
		w.WriteHeader(resp.status)
		_, _ = w.Write(resp.body)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	if resp.stream.cut {
		// Declare a body far larger than we send. When the handler
		// returns, net/http closes the connection instead of ending
		// the body, and the client sees an unexpected EOF mid-stream.
		w.Header().Set("Content-Length", "1073741824")
	}
	w.WriteHeader(resp.status)
	fl, _ := w.(http.Flusher)
	writeStream(w, fl, resp.stream)
}

// endpointKind detects which API family a request targets.
type endpointKind int

const (
	kindOpenAI endpointKind = iota
	kindAnthropic
)

func endpointOf(r *http.Request) endpointKind {
	if strings.HasSuffix(r.URL.Path, "/messages") {
		return kindAnthropic
	}
	return kindOpenAI
}

// requestModel extracts the "model" field of the request body, if any.
func requestModel(reqBody []byte) string {
	var v struct {
		Model string `json:"model"`
	}
	_ = json.Unmarshal(reqBody, &v)
	return v.Model
}

// Text answers with a fixed assistant message, shaped for the matched
// endpoint (OpenAI chat completion or Anthropic message).
func Text(s string) Responder {
	return textResponder(s)
}

type textResponder string

func (t textResponder) respond(r *http.Request, reqBody []byte) response {
	text := string(t)
	model := requestModel(reqBody)
	if endpointOf(r) == kindAnthropic {
		return response{status: 200, header: jsonHeader(), body: anthropicMessageJSON(model, text)}
	}
	return response{status: 200, header: jsonHeader(), body: openAIChatJSON(model, text)}
}

// Stream answers with an SSE stream emitting the chunks in order,
// shaped for the matched endpoint.
func Stream(chunks ...string) Responder {
	return streamResponder(chunks)
}

type streamResponder []string

func (s streamResponder) respond(r *http.Request, _ []byte) response {
	format := formatOpenAI
	if endpointOf(r) == kindAnthropic {
		format = formatAnthropic
	}
	return response{
		status: 200,
		header: http.Header{},
		stream: &streamSpec{format: format, chunks: append([]string(nil), s...)},
	}
}

// Cut answers like Stream but the connection dies after the given
// chunks: no terminating event, no clean end of body. Use it to test
// how your code recovers when a provider drops mid-stream.
func Cut(chunks ...string) Responder {
	return cutResponder(chunks)
}

type cutResponder []string

func (c cutResponder) respond(r *http.Request, _ []byte) response {
	format := formatOpenAI
	if endpointOf(r) == kindAnthropic {
		format = formatAnthropic
	}
	return response{
		status: 200,
		header: http.Header{},
		stream: &streamSpec{format: format, chunks: append([]string(nil), c...), cut: true},
	}
}

// Fail registers a catch-all rule answering every request with the given
// HTTP status. Register it before other rules to fail everything, or
// after On rules with BodyContains to fail specific calls.
func (s *Server) Fail(status int) *FailBuilder {
	f := &FailBuilder{resp: response{status: status, header: jsonHeader(), body: mustJSON(map[string]any{
		"error": map[string]any{"type": "mannequin_injected_failure", "message": fmt.Sprintf("mannequin: injected failure %d", status)},
	})}}
	s.On(func(*http.Request, []byte) bool { return true }, failResponder{f})
	return f
}

// FailBuilder tunes an injected failure.
type FailBuilder struct {
	resp response
}

// WithRetryAfter sets the Retry-After header (as seconds) on the failure.
func (f *FailBuilder) WithRetryAfter(d time.Duration) *FailBuilder {
	f.resp.header.Set("Retry-After", fmt.Sprintf("%d", int(d.Seconds())))
	return f
}

// WithBody replaces the default error body with a raw payload.
func (f *FailBuilder) WithBody(contentType string, body []byte) *FailBuilder {
	f.resp.header.Set("Content-Type", contentType)
	f.resp.body = append([]byte(nil), body...)
	return f
}

type failResponder struct{ f *FailBuilder }

func (fr failResponder) respond(_ *http.Request, _ []byte) response { return fr.f.resp }
