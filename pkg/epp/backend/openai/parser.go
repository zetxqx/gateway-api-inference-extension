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

package openai

import (
	"encoding/json"
	"fmt"
	"strings"

	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/requestcontrol"
)

const (
	streamingRespPrefix = "data: "
	streamingEndMsg     = "data: [DONE]"

	// OpenAI API object types
	objectTypeResponse            = "response"
	objectTypeConversation        = "conversation"
	objectTypeChatCompletion      = "chat.completion"
	objectTypeChatCompletionChunk = "chat.completion.chunk"
	objectTypeTextCompletion      = "text_completion"
)

// Parser implements the backend.Parser interface for OpenAI API.
type Parser struct{}

// NewParser creates a new OpenAI parser.
func NewParser() *Parser {
	return &Parser{}
}

// ParseRequest parses the request body and headers and returns a map representation.
func (p *Parser) ParseRequest(body []byte, headers map[string]string) (map[string]any, error) {
	var requestBody map[string]any
	if err := json.Unmarshal(body, &requestBody); err != nil {
		return nil, fmt.Errorf("error unmarshalling request body: %w", err)
	}
	return requestBody, nil
}

// ParseResponse parses the response body and returns a map representation and usage statistics.
func (p *Parser) ParseResponse(body []byte) (map[string]any, requestcontrol.Usage, error) {
	var responseBody map[string]any
	if err := json.Unmarshal(body, &responseBody); err != nil {
		return nil, requestcontrol.Usage{}, fmt.Errorf("error unmarshalling response body: %w", err)
	}

	var usage requestcontrol.Usage
	if responseBody["usage"] != nil {
		usg := responseBody["usage"].(map[string]any)
		objectType, _ := responseBody["object"].(string)
		usage = extractUsageByAPIType(usg, objectType)
		if usg["prompt_token_details"] != nil {
			detailsMap := usg["prompt_token_details"].(map[string]any)
			if cachedTokens, ok := detailsMap["cached_tokens"]; ok {
				usage.PromptTokenDetails = &requestcontrol.PromptTokenDetails{
					CachedTokens: int(cachedTokens.(float64)),
				}
			}
		}
	}
	return responseBody, usage, nil
}

// ParseStreamResponse parses a chunk of the streaming response and returns usage statistics and a boolean indicating if the stream is complete.
func (p *Parser) ParseStreamResponse(chunk []byte) (requestcontrol.Usage, bool, error) {
	responseText := string(chunk)
	var usage requestcontrol.Usage
	var isComplete bool

	// Parse usage on EVERY chunk to catch split streams (where usage and [DONE] are in different chunks).
	resp := parseRespForUsage(responseText)
	if resp.Usage.TotalTokens > 0 {
		usage = resp.Usage
	}

	if strings.Contains(responseText, streamingEndMsg) {
		isComplete = true
	}
	return usage, isComplete, nil
}

// extractUsageByAPIType extracts usage statistics using the appropriate field names
// based on the OpenAI API type identified by the "object" field.
func extractUsageByAPIType(usg map[string]any, objectType string) requestcontrol.Usage {
	usage := requestcontrol.Usage{}

	switch {
	case strings.HasPrefix(objectType, objectTypeResponse) || strings.HasPrefix(objectType, objectTypeConversation):
		// Responses/Conversations APIs use input_tokens/output_tokens
		if usg["input_tokens"] != nil {
			usage.PromptTokens = int(usg["input_tokens"].(float64))
		}
		if usg["output_tokens"] != nil {
			usage.CompletionTokens = int(usg["output_tokens"].(float64))
		}
	case objectType == objectTypeChatCompletion || objectType == objectTypeChatCompletionChunk || objectType == objectTypeTextCompletion:
		// Traditional APIs use prompt_tokens/completion_tokens
		if usg["prompt_tokens"] != nil {
			usage.PromptTokens = int(usg["prompt_tokens"].(float64))
		}
		if usg["completion_tokens"] != nil {
			usage.CompletionTokens = int(usg["completion_tokens"].(float64))
		}
	default:
		// Fallback: try both field naming conventions
		if usg["input_tokens"] != nil {
			usage.PromptTokens = int(usg["input_tokens"].(float64))
		} else if usg["prompt_tokens"] != nil {
			usage.PromptTokens = int(usg["prompt_tokens"].(float64))
		}

		if usg["output_tokens"] != nil {
			usage.CompletionTokens = int(usg["output_tokens"].(float64))
		} else if usg["completion_tokens"] != nil {
			usage.CompletionTokens = int(usg["completion_tokens"].(float64))
		}
	}

	// total_tokens field name is consistent across all API types
	if usg["total_tokens"] != nil {
		usage.TotalTokens = int(usg["total_tokens"].(float64))
	}

	return usage
}

// Example message if "stream_options": {"include_usage": "true"} is included in the request:
// data: {"id":"...","object":"text_completion","created":1739400043,"model":"small-segment-lora-0","choices":[],
// "usage":{"prompt_tokens":7,"total_tokens":17,"completion_tokens":10}}
//
// data: [DONE]
//
// Noticed that vLLM returns two entries in one response.
// We need to strip the `data:` prefix and next Data: [DONE] from the message to fetch response data.
//
// If include_usage is not included in the request, `data: [DONE]` is returned separately, which
// indicates end of streaming.
func parseRespForUsage(responseText string) ResponseBody {
	response := ResponseBody{}

	lines := strings.SplitSeq(responseText, "\n")
	for line := range lines {
		if !strings.HasPrefix(line, streamingRespPrefix) {
			continue
		}
		content := strings.TrimPrefix(line, streamingRespPrefix)
		if content == "[DONE]" {
			continue
		}

		byteSlice := []byte(content)
		// Ignore errors here as we are parsing best effort
		_ = json.Unmarshal(byteSlice, &response)
	}

	return response
}

type ResponseBody struct {
	Usage requestcontrol.Usage `json:"usage"`
}
