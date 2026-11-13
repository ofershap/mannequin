package mannequin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func post(t *testing.T, url, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer FAKE-SECRET-KEY-12345")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestTextOpenAI(t *testing.T) {
	s := New(t)
	s.On(Chat(), Text("hello from the stand-in"))

	resp := post(t, s.URL()+"/v1/chat/completions", `{"model":"gpt-5.2","messages":[{"role":"user","content":"hi"}]}`)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var got struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(readBody(t, resp)), &got); err != nil {
		t.Fatal(err)
	}
	if got.Choices[0].Message.Content != "hello from the stand-in" {
		t.Fatalf("content = %q", got.Choices[0].Message.Content)
	}
	if got.Model != "gpt-5.2" {
		t.Fatalf("model not echoed: %q", got.Model)
	}
	if got.Choices[0].FinishReason != "stop" {
		t.Fatalf("finish_reason = %q", got.Choices[0].FinishReason)
	}
}

func TestTextAnthropic(t *testing.T) {
	s := New(t)
	s.On(Messages(), Text("שלום מהבובה"))

	resp := post(t, s.URL()+"/v1/messages", `{"model":"claude-sonnet-4-5","max_tokens":100,"messages":[{"role":"user","content":"hi"}]}`)
	var got struct {
		Model   string `json:"model"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
	}
	if err := json.Unmarshal([]byte(readBody(t, resp)), &got); err != nil {
		t.Fatal(err)
	}
	if got.Content[0].Text != "שלום מהבובה" || got.Content[0].Type != "text" {
		t.Fatalf("content = %+v", got.Content[0])
	}
	if got.Model != "claude-sonnet-4-5" || got.StopReason != "end_turn" {
		t.Fatalf("model/stop = %q %q", got.Model, got.StopReason)
	}
}

func TestStreamOpenAI(t *testing.T) {
	s := New(t)
	s.On(Chat(), Stream("Hello, ", "world", "!"))

	resp := post(t, s.URL()+"/v1/chat/completions", `{"model":"gpt-5.2","stream":true,"messages":[]}`)
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content-type = %q", ct)
	}
	body := readBody(t, resp)
	for _, want := range []string{`"content":"Hello, "`, `"content":"world"`, `"content":"!"`, `"finish_reason":"stop"`, "data: [DONE]"} {
		if !strings.Contains(body, want) {
			t.Fatalf("stream missing %q in:\n%s", want, body)
		}
	}
}

func TestStreamAnthropic(t *testing.T) {
	s := New(t)
	s.On(Messages(), Stream("a", "b"))

	resp := post(t, s.URL()+"/v1/messages", `{"model":"claude-sonnet-4-5","stream":true,"messages":[]}`)
	body := readBody(t, resp)
	for _, want := range []string{"event: message_start", `"type":"text_delta"`, `"text":"a"`, "event: content_block_stop", "event: message_delta", "event: message_stop"} {
		if !strings.Contains(body, want) {
			t.Fatalf("stream missing %q in:\n%s", want, body)
		}
	}
}

func TestFail429(t *testing.T) {
	s := New(t)
	s.Fail(429).WithRetryAfter(time.Second)

	resp := post(t, s.URL()+"/v1/chat/completions", `{"model":"gpt-5.2","messages":[]}`)
	if resp.StatusCode != 429 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if ra := resp.Header.Get("Retry-After"); ra != "1" {
		t.Fatalf("Retry-After = %q", ra)
	}
}

func TestUnmatched404(t *testing.T) {
	s := New(t)
	resp := post(t, s.URL()+"/v1/chat/completions", `{"model":"x","messages":[]}`)
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if body := readBody(t, resp); !strings.Contains(body, "mannequin_unmatched_request") {
		t.Fatalf("body = %s", body)
	}
}

func TestBodyContains(t *testing.T) {
	s := New(t)
	s.On(BodyContains(Chat(), "gpt-5.2"), Text("five"))
	s.On(Chat(), Text("other"))

	resp := post(t, s.URL()+"/v1/chat/completions", `{"model":"gpt-5.2","messages":[]}`)
	if body := readBody(t, resp); !strings.Contains(body, "five") {
		t.Fatalf("body = %s", body)
	}
	resp = post(t, s.URL()+"/v1/chat/completions", `{"model":"gpt-4.1","messages":[]}`)
	if body := readBody(t, resp); !strings.Contains(body, "other") {
		t.Fatalf("body = %s", body)
	}
}

func TestRecordReplay(t *testing.T) {
	cassette := filepath.Join(t.TempDir(), "cassette.json")
	const reqBody = `{"model":"gpt-5.2","messages":[{"role":"user","content":"ping"}]}`

	rec := New(nil)
	rec.Record(cassette)
	rec.On(Chat(), Text("pong"))
	resp := post(t, rec.URL()+"/v1/chat/completions", reqBody)
	readBody(t, resp)
	rec.Close() // triggers save

	data, err := os.ReadFile(cassette)
	if err != nil {
		t.Fatal(err)
	}
	// Credentials must never reach the cassette.
	if strings.Contains(string(data), "FAKE-SECRET-KEY-12345") {
		t.Fatal("cassette leaked the Authorization credential")
	}
	if strings.Contains(string(data), "authorization") || strings.Contains(string(data), "Authorization") {
		t.Fatal("cassette contains header material")
	}

	player := New(t)
	replay, err := Replay(cassette)
	if err != nil {
		t.Fatal(err)
	}
	player.On(Path(http.MethodPost, "/chat/completions"), replay)

	resp = post(t, player.URL()+"/v1/chat/completions", reqBody)
	if resp.StatusCode != 200 {
		t.Fatalf("replay status = %d", resp.StatusCode)
	}
	if body := readBody(t, resp); !strings.Contains(body, "pong") {
		t.Fatalf("replay body = %s", body)
	}

	resp = post(t, player.URL()+"/v1/chat/completions", `{"model":"gpt-5.2","messages":[{"role":"user","content":"DIFFERENT"}]}`)
	if resp.StatusCode != 422 {
		t.Fatalf("miss status = %d", resp.StatusCode)
	}
}

func TestStreamingIsNotRecorded(t *testing.T) {
	cassette := filepath.Join(t.TempDir(), "stream.json")
	rec := New(nil)
	rec.Record(cassette)
	rec.On(Chat(), Stream("secret-chunk"))
	resp := post(t, rec.URL()+"/v1/chat/completions", `{"model":"gpt-5.2","stream":true,"messages":[]}`)
	readBody(t, resp)
	rec.Close()

	data, err := os.ReadFile(cassette)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret-chunk") {
		t.Fatal("streaming response leaked into cassette")
	}
}

func TestScrubHook(t *testing.T) {
	cassette := filepath.Join(t.TempDir(), "scrub.json")
	rec := New(nil)
	rec.Record(cassette, func(ex *Exchange) {
		ex.RequestBody = json.RawMessage(`"scrubbed"`)
	})
	rec.On(Chat(), Text("pong"))
	resp := post(t, rec.URL()+"/v1/chat/completions", `{"model":"gpt-5.2","messages":[{"role":"user","content":"PRIVATE-PROMPT"}]}`)
	readBody(t, resp)
	rec.Close()

	data, err := os.ReadFile(cassette)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "PRIVATE-PROMPT") {
		t.Fatal("scrub hook did not censor the request body")
	}
}

func TestConcurrent(t *testing.T) {
	s := New(t)
	s.On(Chat(), Text("ok"))
	s.On(Messages(), Text("ok"))

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			path := "/v1/chat/completions"
			body := `{"model":"m","messages":[]}`
			if i%2 == 1 {
				path = "/v1/messages"
			}
			resp := post(t, s.URL()+path, body)
			readBody(t, resp)
		}(i)
	}
	wg.Wait()
}

func Example() {
	s := New(nil)
	defer s.Close()
	s.On(Chat(), Text("42"))

	resp, err := http.Post(s.URL()+"/v1/chat/completions", "application/json", strings.NewReader(`{"model":"gpt-5.2","messages":[{"role":"user","content":"meaning?"}]}`))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	var got struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	json.NewDecoder(resp.Body).Decode(&got)
	fmt.Println(got.Choices[0].Message.Content)
	// Output: 42
}
