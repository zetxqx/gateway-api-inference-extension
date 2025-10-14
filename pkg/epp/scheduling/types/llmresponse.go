/*
Copyright 2025 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"encoding/json"
	"fmt"
)

// LLMResponse is a structured representation of a parsed LLM response body.
// An LLMResponse must contain exactly one of ChatCompletion or LegacyCompletion.
type LLMResponse struct {
	// ChatCompletion is the representation of the OpenAI /v1/chat/completions response body.
	ChatCompletion *ChatCompletionResponse `json:"chat_completion,omitempty"`
	// LegacyCompletion is the representation of the OpenAI /v1/completions response body.
	LegacyCompletion *LegacyCompletionResponse `json:"legacy_completion,omitempty"`
}

// ChatCompletionResponse represents the full response body for the chat completions API.
type ChatCompletionResponse struct {
	Choices []ChatChoice `json:"choices"`
	Usage   *Usage       `json:"usage,omitempty"`
}

func (r *ChatCompletionResponse) String() string {
	if r == nil {
		return nilString
	}
	contentLen := 0
	if len(r.Choices) > 0 {
		contentLen = len(r.Choices[0].Message.Content)
	}
	return fmt.Sprintf("{ContentLength: %d, Usage: %s}", contentLen, r.Usage)
}

// ChatChoice represents a single choice in the chat completion response.
type ChatChoice struct {
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// ChatMessage represents the message object within a choice.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// LegacyCompletionResponse represents the full response body for the legacy completions API.
type LegacyCompletionResponse struct {
	Choices []LegacyChoice `json:"choices"`
	Usage   *Usage         `json:"usage,omitempty"`
}

func (r *LegacyCompletionResponse) String() string {
	if r == nil {
		return nilString
	}
	textLen := 0
	if len(r.Choices) > 0 {
		textLen = len(r.Choices[0].Text)
	}
	return fmt.Sprintf("{TextLength: %d, Usage: %v}", textLen, r.Usage)
}

// LegacyChoice represents a single choice in the legacy completion response.
type LegacyChoice struct {
	Text         string `json:"text"`
	FinishReason string `json:"finish_reason"`
}

// Usage represents the token usage data common to all response formats.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (u *Usage) String() string {
	if u == nil {
		return nilString
	}
	return fmt.Sprintf("{Prompt: %d, Completion: %d, Total: %d}", u.PromptTokens, u.CompletionTokens, u.TotalTokens)
}

// NewLLMResponseFromBytes initializes an LLMResponse by trying to parse the data
// as a chat completion and then as a legacy completion response.
func NewLLMResponseFromBytes(body []byte) (*LLMResponse, error) {
	if len(body) == 0 {
		return nil, fmt.Errorf("input bytes are empty")
	}

	// Attempt to unmarshal as a ChatCompletionResponse first.
	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(body, &chatResp); err == nil {
		// Check if the role is set to distinguish ChatCompletion and LegacyCompletion.
		if len(chatResp.Choices) > 0 && chatResp.Choices[0].Message.Role != "" {
			return &LLMResponse{ChatCompletion: &chatResp}, nil
		}
	}

	// Try to unmarshal as a LegacyCompletionResponse.
	var legacyResp LegacyCompletionResponse
	if err := json.Unmarshal(body, &legacyResp); err == nil {
		if len(legacyResp.Choices) > 0 {
			return &LLMResponse{LegacyCompletion: &legacyResp}, nil
		}
	}

	return nil, fmt.Errorf("failed to unmarshal body into any known LLM response format")
}
