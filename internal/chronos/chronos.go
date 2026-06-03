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

// regex to split by word boundaries while preserving all whitespace/newlines
var chunker = regexp.MustCompile(`\S+\s*|\s+`)

func StreamOpenAI(w http.ResponseWriter, flusher http.Flusher, modelRequested string, fullText string) {
	time.Sleep(500 * time.Millisecond) // TTFT

	streamID := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
	chunks := chunker.FindAllString(fullText, -1)

	for _, chunkText := range chunks {
		chunk := map[string]interface{}{
			"id":      streamID,
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   modelRequested,
			"choices": []map[string]interface{}{
				{"delta": map[string]string{"content": chunkText}},
			},
		}

		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()

		// Jitter logic (skip jitter for pure whitespace)
		if strings.TrimSpace(chunkText) != "" {
			delay := 20 + rand.Intn(30)
			if strings.HasSuffix(strings.TrimSpace(chunkText), ".") {
				delay += 80
			}
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}
	}
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}
