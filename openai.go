package mannequin

import (
	"fmt"
	"io"
	"net/http"
)

// OpenAI response shapes (chat completions).

func openAIChatJSON(model, text string) []byte {
	return mustJSON(map[string]any{
		"id":      "chatcmpl-mannequin",
		"object":  "chat.completion",
		"created": 1700000000,
		"model":   model,
		"choices": []any{map[string]any{
			"index":         0,
			"message":       map[string]any{"role": "assistant", "content": text},
			"finish_reason": "stop",
		}},
		"usage": map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
	})
}

func openAIChunkJSON(model, role, content string, finish string) []byte {
	delta := map[string]any{}
	if role != "" {
		delta["role"] = role
	}
	if content != "" {
		delta["content"] = content
	}
	var finishVal any
	if finish != "" {
		finishVal = finish
	}
	return mustJSON(map[string]any{
		"id":      "chatcmpl-mannequin",
		"object":  "chat.completion.chunk",
		"created": 1700000000,
		"model":   model,
		"choices": []any{map[string]any{
			"index":         0,
			"delta":         delta,
			"finish_reason": finishVal,
		}},
	})
}

func writeOpenAIStream(w io.Writer, fl http.Flusher, chunks []string) {
	model := ""
	// emit writes one chunk and flushes; a write error means the caller is
	// gone, so the stream stops early.
	emit := func(s string) bool {
		_, err := io.WriteString(w, s)
		flush(fl)
		return err == nil
	}
	if !emit(fmt.Sprintf("data: %s\n\n", openAIChunkJSON(model, "assistant", "", ""))) {
		return
	}
	for _, c := range chunks {
		if !emit(fmt.Sprintf("data: %s\n\n", openAIChunkJSON(model, "", c, ""))) {
			return
		}
	}
	if !emit(fmt.Sprintf("data: %s\n\n", openAIChunkJSON(model, "", "", "stop"))) {
		return
	}
	emit("data: [DONE]\n\n")
}

func flush(fl http.Flusher) {
	if fl != nil {
		fl.Flush()
	}
}
