package chronos

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var chunker = regexp.MustCompile(`\S+\s*|\s+`)

// StreamOpenAI writes a spec-compliant OpenAI SSE stream.
func StreamOpenAI(w http.ResponseWriter, flusher http.Flusher, model string, fullText string) {
	time.Sleep(500 * time.Millisecond)

	id := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
	ts := time.Now().Unix()

	// First chunk: role announcement (matches real OpenAI behaviour)
	emitChunk(w, flusher, id, model, ts, map[string]interface{}{"role": "assistant", "content": ""}, nil)

	for _, tok := range chunker.FindAllString(fullText, -1) {
		emitChunk(w, flusher, id, model, ts, map[string]interface{}{"content": tok}, nil)

		if strings.TrimSpace(tok) != "" {
			delay := 20 + rand.Intn(30)
			if strings.HasSuffix(strings.TrimSpace(tok), ".") {
				delay += 80
			}
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}
	}

	// Final chunk: finish_reason stop, empty delta
	stop := "stop"
	emitChunk(w, flusher, id, model, ts, map[string]interface{}{}, &stop)

	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

// BuildOpenAI returns a spec-compliant non-streaming chat.completion object.
func BuildOpenAI(model string, fullText string) map[string]interface{} {
	completionTokens := len(strings.Fields(fullText))
	promptTokens := 12
	return map[string]interface{}{
		"id":                 fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		"object":             "chat.completion",
		"created":            time.Now().Unix(),
		"model":              model,
		"system_fingerprint": "fp_llmplaceholder",
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]string{
					"role":    "assistant",
					"content": fullText,
				},
				"finish_reason": "stop",
				"logprobs":      nil,
			},
		},
		"usage": map[string]int{
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
			"total_tokens":      promptTokens + completionTokens,
		},
	}
}

// BuildAnthropic returns a spec-compliant non-streaming Anthropic messages response.
func BuildAnthropic(model string, fullText string) map[string]interface{} {
	outputTokens := len(strings.Fields(fullText))
	inputTokens := 12
	return map[string]interface{}{
		"id":            fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		"type":          "message",
		"role":          "assistant",
		"content":       []map[string]interface{}{{"type": "text", "text": fullText}},
		"model":         model,
		"stop_reason":   "end_turn",
		"stop_sequence": nil,
		"usage": map[string]int{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
		},
	}
}

// StreamAnthropic writes a spec-compliant Anthropic SSE stream.
func StreamAnthropic(w http.ResponseWriter, flusher http.Flusher, model string, fullText string) {
	time.Sleep(500 * time.Millisecond)

	msgID := fmt.Sprintf("msg_%d", time.Now().UnixNano())
	outputTokens := len(strings.Fields(fullText))

	emitAnthropicEvent(w, flusher, "message_start", map[string]interface{}{
		"type": "message_start",
		"message": map[string]interface{}{
			"id":            msgID,
			"type":          "message",
			"role":          "assistant",
			"content":       []interface{}{},
			"model":         model,
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage":         map[string]int{"input_tokens": 12, "output_tokens": 1},
		},
	})

	emitAnthropicEvent(w, flusher, "content_block_start", map[string]interface{}{
		"type":          "content_block_start",
		"index":         0,
		"content_block": map[string]string{"type": "text", "text": ""},
	})

	emitAnthropicEvent(w, flusher, "ping", map[string]interface{}{"type": "ping"})

	for _, tok := range chunker.FindAllString(fullText, -1) {
		emitAnthropicEvent(w, flusher, "content_block_delta", map[string]interface{}{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]string{"type": "text_delta", "text": tok},
		})

		if strings.TrimSpace(tok) != "" {
			delay := 20 + rand.Intn(30)
			if strings.HasSuffix(strings.TrimSpace(tok), ".") {
				delay += 80
			}
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}
	}

	emitAnthropicEvent(w, flusher, "content_block_stop", map[string]interface{}{
		"type":  "content_block_stop",
		"index": 0,
	})

	emitAnthropicEvent(w, flusher, "message_delta", map[string]interface{}{
		"type":  "message_delta",
		"delta": map[string]interface{}{"stop_reason": "end_turn", "stop_sequence": nil},
		"usage": map[string]int{"output_tokens": outputTokens},
	})

	emitAnthropicEvent(w, flusher, "message_stop", map[string]interface{}{"type": "message_stop"})
}

func emitAnthropicEvent(w http.ResponseWriter, flusher http.Flusher, event string, payload interface{}) {
	data, _ := json.Marshal(payload)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	flusher.Flush()
}

func emitChunk(w http.ResponseWriter, flusher http.Flusher, id, model string, ts int64, delta map[string]interface{}, finishReason *string) {
	chunk := map[string]interface{}{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": ts,
		"model":   model,
		"choices": []map[string]interface{}{
			{
				"index":         0,
				"delta":         delta,
				"logprobs":      nil,
				"finish_reason": finishReason,
			},
		},
	}
	data, _ := json.Marshal(chunk)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}
