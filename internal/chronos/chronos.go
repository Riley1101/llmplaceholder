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
