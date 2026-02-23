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

package vllm

import (
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	vllmpb "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/api/gen"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/backend"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/requestcontrol"
)

// Parser implements the backend.Parser interface for vLLM Protocol.
type Parser struct{}

// NewParser creates a new vLLM parser.
func NewParser() *Parser {
	return &Parser{}
}

// VLLMRequest implements BackendRequest for vLLM.
type VLLMRequest struct {
	req     *vllmpb.GenerateRequest
	headers map[string]string
}

func (r *VLLMRequest) Get(key string) (any, bool) {
	if key == "model" {
		if model, ok := r.headers["x-model-name"]; ok {
			return model, true
		}
		if model, ok := r.headers["model"]; ok {
			return model, true
		}
		return nil, false
	}
	if key == "prompt" {
		switch v := r.req.Input.(type) {
		case *vllmpb.GenerateRequest_Text:
			return v.Text, true
		case *vllmpb.GenerateRequest_Tokenized:
			return v.Tokenized.OriginalText, true
		}
		return nil, false
	}
	if key == "stream" {
		return r.req.Stream, true
	}
	return nil, false
}

func (r *VLLMRequest) Set(key string, value any) error {
	if key == "model" {
		// Update header for model
		if strVal, ok := value.(string); ok {
			r.headers["x-model-name"] = strVal
			return nil
		}
		return fmt.Errorf("invalid value type for model: %T", value)
	}
	if key == "prompt" {
		if strVal, ok := value.(string); ok {
			r.req.Input = &vllmpb.GenerateRequest_Text{Text: strVal}
			return nil
		}
		return fmt.Errorf("invalid value type for prompt: %T", value)
	}
	return nil
}

func (r *VLLMRequest) Marshal() ([]byte, error) {
	return proto.Marshal(r.req)
}

func (r *VLLMRequest) ToMap() map[string]any {
	// Construct a map representation
	requestMap := make(map[string]any)

	// Populate model
	if val, ok := r.Get("model"); ok {
		requestMap["model"] = val
	}

	// Populate prompt
	if val, ok := r.Get("prompt"); ok {
		requestMap["prompt"] = val
	}

	// Populate stream
	requestMap["stream"] = r.req.Stream

	return requestMap
}

// ParseRequest parses the request body and headers and returns a BackendRequest object.
func (p *Parser) ParseRequest(body []byte, headers map[string]string) (backend.BackendRequest, error) {
	var req vllmpb.GenerateRequest
	if err := proto.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("error unmarshalling vllm request body: %w", err)
	}

	// We use the passed headers map directly so that modifications (like Set("model", ...))
	// are reflected in the map held by the caller (StreamingServer), ensuring they propagate upstream.
	return &VLLMRequest{req: &req, headers: headers}, nil
}

// ParseResponse parses the response body and returns a map representation and usage statistics.
func (p *Parser) ParseResponse(body []byte) (map[string]any, requestcontrol.Usage, error) {
	// For gRPC/Protobuf, the response body handled here would be a single message if non-streaming.
	var resp vllmpb.GenerateResponse
	if err := proto.Unmarshal(body, &resp); err != nil {
		return nil, requestcontrol.Usage{}, fmt.Errorf("error unmarshalling vllm response body: %w", err)
	}

	usage := requestcontrol.Usage{}
	if complete := resp.GetComplete(); complete != nil {
		usage.PromptTokens = int(complete.PromptTokens)
		usage.CompletionTokens = int(complete.CompletionTokens)
		usage.TotalTokens = int(complete.PromptTokens + complete.CompletionTokens)
		if complete.CachedTokens > 0 {
			usage.PromptTokenDetails = &requestcontrol.PromptTokenDetails{
				CachedTokens: int(complete.CachedTokens),
			}
		}
	}

	// Marshal to JSON then Unmarshal to map to provide the full response structure
	// This ensures plugins and other components have access to all fields (e.g., output_ids).
	jsonBytes, err := protojson.Marshal(&resp)
	if err != nil {
		return nil, requestcontrol.Usage{}, fmt.Errorf("error marshalling response to json: %w", err)
	}

	var respMap map[string]any
	if err := json.Unmarshal(jsonBytes, &respMap); err != nil {
		return nil, requestcontrol.Usage{}, fmt.Errorf("error unmarshalling json to map: %w", err)
	}

	return respMap, usage, nil
}

// ParseStreamResponse parses a chunk of the streaming response and returns usage statistics and a boolean indicating if the stream is complete.
func (p *Parser) ParseStreamResponse(chunk []byte) (map[string]any, requestcontrol.Usage, bool, error) {
	var resp vllmpb.GenerateResponse
	if err := proto.Unmarshal(chunk, &resp); err != nil {
		return nil, requestcontrol.Usage{}, false, fmt.Errorf("error unmarshalling vllm stream chunk: %w", err)
	}

	usage := requestcontrol.Usage{}
	isComplete := false

	if c := resp.GetChunk(); c != nil {
		// Stream chunk
		usage.PromptTokens = int(c.PromptTokens)
		usage.CompletionTokens = int(c.CompletionTokens)
		usage.TotalTokens = int(c.PromptTokens + c.CompletionTokens)
		if c.CachedTokens > 0 {
			usage.PromptTokenDetails = &requestcontrol.PromptTokenDetails{
				CachedTokens: int(c.CachedTokens),
			}
		}
	} else if c := resp.GetComplete(); c != nil {
		// Final completion message
		isComplete = true
		usage.PromptTokens = int(c.PromptTokens)
		usage.CompletionTokens = int(c.CompletionTokens)
		usage.TotalTokens = int(c.PromptTokens + c.CompletionTokens)
		if c.CachedTokens > 0 {
			usage.PromptTokenDetails = &requestcontrol.PromptTokenDetails{
				CachedTokens: int(c.CachedTokens),
			}
		}
	}

	jsonBytes, err := protojson.Marshal(&resp)
	if err != nil {
		return nil, requestcontrol.Usage{}, false, fmt.Errorf("error marshalling response to json: %w", err)
	}

	var respMap map[string]any
	if err := json.Unmarshal(jsonBytes, &respMap); err != nil {
		return nil, requestcontrol.Usage{}, false, fmt.Errorf("error unmarshalling json to map: %w", err)
	}

	return respMap, usage, isComplete, nil
}
