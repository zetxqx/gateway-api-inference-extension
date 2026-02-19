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

package parsers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/parsers"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/plugin"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/requestcontrol"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/scheduling"
	requtil "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/util/request"
)

const (
	PluginName = "openai_parser"
	PluginType = "request_parser"
)

// Ensure implementation
var _ parsers.Parser = &OpenAIParser{}

// OpenAIParser implements the RequestBodyParser interface for OpenAI HTTP protocol.
type OpenAIParser struct{}

// NewOpenAIParser creates a new OpenAIParser.
func NewOpenAIParser(name string, parameters json.RawMessage, handle plugin.Handle) (plugin.Plugin, error) {
	return &OpenAIParser{}, nil
}

func (p *OpenAIParser) TypedName() plugin.TypedName {
	return plugin.TypedName{
		Type: PluginType,
		Name: PluginName,
	}
}

// Matches returns true if the request looks like an OpenAI request (JSON content type).
func (p *OpenAIParser) Matches(headers map[string]string, body []byte) *parsers.MatchResult {
	contentType, ok := headers["content-type"]
	if !ok {
		return nil
	}
	return &parsers.MatchResult{
		Matched: strings.Contains(contentType, "application/json"),
	}
}

func (p *OpenAIParser) ExtractSchedulingContext(ctx context.Context, headers map[string]string, body []byte) (*scheduling.LLMRequest, error) {
	var bodyMap map[string]interface{}
	if err := json.Unmarshal(body, &bodyMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal request body: %w", err)
	}

	model, _ := bodyMap["model"].(string)

	llmBody, err := requtil.ExtractRequestBody(bodyMap, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to extract LLM request body: %w", err)
	}

	reqID := headers[requtil.RequestIdHeaderKey]

	return &scheduling.LLMRequest{
		RequestId:   reqID,
		TargetModel: model,
		Body:        llmBody,
		Headers:     headers,
	}, nil
}

func (p *OpenAIParser) ExtractUsage(ctx context.Context, headers map[string]string, respBody []byte) (*requestcontrol.Usage, error) {
	return &requestcontrol.Usage{}, nil
}
