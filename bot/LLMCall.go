package bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type OllamaOptions struct {
	Temperature float64 `json:"temperature"`
	NumCtx      int     `json:"num_ctx"`
}

type OllamaRequest struct {
	Model   string        `json:"model"`
	Prompt  string        `json:"prompt"`
	Stream  bool          `json:"stream"`
	Options OllamaOptions `json:"options"`
}

type OllamaResponse struct {
	Response string `json:"response"`
}

type QueryConfig struct {
	Model       string
	Temperature float64
	NumCtx      int
}

func SQLConfig() QueryConfig {
	return QueryConfig{
		Model:       "qwen2.5-coder:14b",
		Temperature: 0.1,
		NumCtx:      8192,
	}
}

func ClassifierConfig() QueryConfig {
	return QueryConfig{
		Model:       "qwen2.5:7b",
		Temperature: 0.0,
		NumCtx:      4096,
	}
}

func RoleplayConfig() QueryConfig {
	return QueryConfig{
		Model:       "mistral-nemo",
		Temperature: 0.7,
		NumCtx:      8192,
	}
}

func SummarizerConfig() QueryConfig {
	return QueryConfig{
		Model:       "qwen2.5:7b",
		Temperature: 0.1,
		NumCtx:      4096,
	}
}

const maxQueryAttempts = 3

// Query sends a prompt to Ollama with automatic retries on failure.
// Attempts: 1 immediate, then 1s delay, then 2s delay.
func Query(prompt string, config QueryConfig) (string, error) {
	var lastErr error
	for i := 0; i < maxQueryAttempts; i++ {
		if i > 0 {
			delay := time.Duration(1<<uint(i-1)) * time.Second // 1s, 2s
			log.Printf("Query retry %d/%d for model %s (waiting %s): %v",
				i+1, maxQueryAttempts, config.Model, delay, lastErr)
			time.Sleep(delay)
		}

		resp, err := queryOnce(prompt, config)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}
	return "", fmt.Errorf("all %d attempts failed (model: %s): %w", maxQueryAttempts, config.Model, lastErr)
}

// queryOnce makes a single attempt to the Ollama API.
func queryOnce(prompt string, config QueryConfig) (string, error) {
	reqBody := OllamaRequest{
		Model:  config.Model,
		Prompt: prompt,
		Stream: false,
		Options: OllamaOptions{
			Temperature: config.Temperature,
			NumCtx:      config.NumCtx,
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to reach ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var ollamaResp OllamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return ollamaResp.Response, nil
}

// QueryWithSchema builds a structured D&D SQL prompt and queries Ollama.
// schema:   the relevant table definitions for this batch
// roleplay: the current D&D scene text
func QueryWithSchema(schema string, roleplay string, config QueryConfig) (string, error) {
	prompt := fmt.Sprintf(`
You are a D&D campaign assistant. Given a database schema and a roleplay scene,
write SQL queries (SQLite) to retrieve all information relevant to formulating
a response to this scene.

## Schema
%s

## Roleplay Scene
%s

## Instructions
- Write only queries that are directly relevant to this scene.
- Wrap each query in a sql code block.
- Add a one-line comment above each query explaining why it is needed.
`, schema, roleplay)

	return Query(prompt, config)
}
