package chronos

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

// StreamOpenAI formats a raw string into OpenAI SSE chunks and applies network jitter
func StreamOpenAI(w http.ResponseWriter, flusher http.Flusher, modelRequested string, fullText string) {
	// 1. Simulate Time-To-First-Token (TTFT)
	time.Sleep(500 * time.Millisecond)

	streamID := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
	words := strings.Split(fullText, " ")

	for i, word := range words {
		chunkText := word
		if i > 0 {
			chunkText = " " + word
		}

		chunk := map[string]interface{}{
			"id":      streamID,
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   modelRequested,
			"choices": []map[string]interface{}{
				{
					"delta": map[string]string{
						"content": chunkText,
					},
				},
			},
		}

		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()

		// 2. Simulate per-token network jitter & punctuation pauses
		delay := 30 + rand.Intn(40)
		if strings.HasSuffix(word, ".") || strings.HasSuffix(word, ",") {
			delay += 100
		}
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}

	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}
