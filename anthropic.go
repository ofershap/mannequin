package mannequin

import (
	"fmt"
	"io"
	"net/http"
)

// Anthropic response shapes (messages API).

func anthropicMessageJSON(model, text string) []byte {
	return mustJSON(map[string]any{
		"id":            "msg_mannequin",
		"type":          "message",
		"role":          "assistant",
		"model":         model,
		"content":       []any{map[string]any{"type": "text", "text": text}},
		"stop_reason":   "end_turn",
		"stop_sequence": nil,
		"usage":         map[string]any{"input_tokens": 1, "output_tokens": 1},
	})
}

func anthropicEvent(event string, data map[string]any) string {
	// Anthropic event bodies carry the event name in their "type" field;
	// SDK unions switch on it.
	data["type"] = event
	return fmt.Sprintf("event: %s\ndata: %s\n\n", event, mustJSON(data))
}

func writeAnthropicStream(w io.Writer, fl http.Flusher, model string, chunks []string, cut bool) {
	// emit writes one event and flushes; a write error means the caller is
	// gone, so the stream stops early.
	emit := func(event string, data map[string]any) bool {
		_, err := io.WriteString(w, anthropicEvent(event, data))
		flush(fl)
		return err == nil
	}
	if !emit("message_start", map[string]any{
		"message": map[string]any{
			"id": "msg_mannequin", "type": "message", "role": "assistant", "model": model,
			"content": []any{}, "stop_reason": nil, "stop_sequence": nil,
			"usage": map[string]any{"input_tokens": 1, "output_tokens": 0},
		},
	}) {
		return
	}
	if !emit("content_block_start", map[string]any{
		"index": 0, "content_block": map[string]any{"type": "text", "text": ""},
	}) {
		return
	}
	for _, c := range chunks {
		if !emit("content_block_delta", map[string]any{
			"index": 0, "delta": map[string]any{"type": "text_delta", "text": c},
		}) {
			return
		}
	}
	if cut {
		return // die mid-stream: no closing events
	}
	if !emit("content_block_stop", map[string]any{"index": 0}) {
		return
	}
	if !emit("message_delta", map[string]any{
		"delta": map[string]any{"stop_reason": "end_turn", "stop_sequence": nil},
		"usage": map[string]any{"output_tokens": len(chunks)},
	}) {
		return
	}
	emit("message_stop", map[string]any{"type": "message_stop"})
}

func writeStream(w io.Writer, fl http.Flusher, spec *streamSpec) {
	switch spec.format {
	case formatAnthropic:
		writeAnthropicStream(w, fl, "", spec.chunks, spec.cut)
	default:
		writeOpenAIStream(w, fl, spec.chunks, spec.cut)
	}
}
