// Compat tests pin mannequin against real SDK versions:
// openai-go v2.7.1, anthropic-sdk-go v1.84.0 (recorded 13 Nov 2026).
package compat

import (
	"context"
	"strings"
	"testing"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	aoption "github.com/anthropics/anthropic-sdk-go/option"
	"github.com/ofershap/mannequin"
	openai "github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

func TestOpenAISDK(t *testing.T) {
	s := mannequin.New(t)
	s.On(mannequin.Chat(), mannequin.Text("shalom"))

	client := openai.NewClient(option.WithBaseURL(s.URL()), option.WithAPIKey("test-key"))
	resp, err := client.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
		Model:    "gpt-5.2",
		Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage("hi")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.Choices[0].Message.Content; got != "shalom" {
		t.Fatalf("content = %q", got)
	}
	if resp.Model != "gpt-5.2" {
		t.Fatalf("model = %q", resp.Model)
	}
}

func TestOpenAISDKStreaming(t *testing.T) {
	s := mannequin.New(t)
	s.On(mannequin.Chat(), mannequin.Stream("Hello, ", "world"))

	client := openai.NewClient(option.WithBaseURL(s.URL()), option.WithAPIKey("test-key"))
	stream := client.Chat.Completions.NewStreaming(context.Background(), openai.ChatCompletionNewParams{
		Model:    "gpt-5.2",
		Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage("hi")},
	})
	var sb strings.Builder
	for stream.Next() {
		chunk := stream.Current()
		if len(chunk.Choices) > 0 {
			sb.WriteString(chunk.Choices[0].Delta.Content)
		}
	}
	if err := stream.Err(); err != nil {
		t.Fatal(err)
	}
	if got := sb.String(); got != "Hello, world" {
		t.Fatalf("streamed = %q", got)
	}
}

func TestAnthropicSDK(t *testing.T) {
	s := mannequin.New(t)
	s.On(mannequin.Messages(), mannequin.Text("בדיקה"))

	client := anthropic.NewClient(aoption.WithBaseURL(s.URL()), aoption.WithAPIKey("test-key"))
	msg, err := client.Messages.New(context.Background(), anthropic.MessageNewParams{
		Model:     "claude-sonnet-4-5",
		MaxTokens: 100,
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock("hi"))},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(msg.Content) == 0 {
		t.Fatal("no content blocks")
	}
	if got := msg.Content[0].AsText().Text; got != "בדיקה" {
		t.Fatalf("text = %q", got)
	}
	if msg.StopReason != "end_turn" {
		t.Fatalf("stop_reason = %q", msg.StopReason)
	}
}
