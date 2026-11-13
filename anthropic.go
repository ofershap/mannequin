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

func anthropicEvent(event string, data any) string {
	return fmt.Sprintf("event: %s\ndata: %s\n\n", event, mustJSON(data))
}

func writeAnthropicStream(w io.Writer, fl http.Flusher, model string, chunks []string) {
	io.WriteString(w, anthropicEvent("message_start", map[string]any{
		"message": map[string]any{
			"id": "msg_mannequin", "type": "message", "role": "assistant", "model": model,
			"content": []any{}, "stop_reason": nil, "stop_sequence": nil,
			"usage": map[string]any{"input_tokens": 1, "output_tokens": 0},
		},
	}))
	flush(fl)
	io.WriteString(w, anthropicEvent("content_block_start", map[string]any{
		"index": 0, "content_block": map[string]any{"type": "text", "text": ""},
	}))
	flush(fl)
	for _, c := range chunks {
		io.WriteString(w, anthropicEvent("content_block_delta", map[string]any{
			"index": 0, "delta": map[string]any{"type": "text_delta", "text": c},
		}))
		flush(fl)
	}
	io.WriteString(w, anthropicEvent("content_block_stop", map[string]any{"index": 0}))
	io.WriteString(w, anthropicEvent("message_delta", map[string]any{
		"delta": map[string]any{"stop_reason": "end_turn", "stop_sequence": nil},
		"usage": map[string]any{"output_tokens": len(chunks)},
	}))
	io.WriteString(w, anthropicEvent("message_stop", map[string]any{"type": "message_stop"}))
	flush(fl)
}

func writeStream(w io.Writer, fl http.Flusher, spec *streamSpec) {
	switch spec.format {
	case formatAnthropic:
		writeAnthropicStream(w, fl, "", spec.chunks)
	default:
		writeOpenAIStream(w, fl, spec.chunks)
	}
}
