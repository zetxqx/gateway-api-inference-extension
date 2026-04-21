/*
Copyright 2026 The Kubernetes Authors.

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

package vertexai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"cloud.google.com/go/aiplatform/apiv1beta1/aiplatformpb"
	"google.golang.org/genproto/googleapis/api/httpbody"
	"google.golang.org/protobuf/proto"
	"sigs.k8s.io/controller-runtime/pkg/log"
	v1 "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	logutil "sigs.k8s.io/gateway-api-inference-extension/pkg/common/observability/logging"
	fwkplugin "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/plugin"
	fwkrh "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/requesthandling"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/plugins/requesthandling/parsers"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/plugins/requesthandling/parsers/openai"
)

const (
	VertexAIParserType = "vertexai-parser"

	chatCompletionsMethod = "PredictionService/ChatCompletions"
)

// compile-time type validation
var _ fwkrh.Parser = &VertexAIParser{}

// VertexAIParser implements the fwkrh.Parser interface for Vertex AI gRPC API
type VertexAIParser struct {
	typedName fwkplugin.TypedName
}

// NewVertexAIParser creates a new VertexAIParser.
func NewVertexAIParser() *VertexAIParser {
	return &VertexAIParser{
		typedName: fwkplugin.TypedName{
			Type: VertexAIParserType,
			Name: VertexAIParserType,
		},
	}
}

// TypedName returns the type and name tuple of this plugin instance.
func (p *VertexAIParser) TypedName() fwkplugin.TypedName {
	return p.typedName
}

func (p *VertexAIParser) SupportedAppProtocols() []v1.AppProtocol {
	return []v1.AppProtocol{v1.AppProtocolH2C}
}

func VertexAIParserPluginFactory(name string, _ json.RawMessage, _ fwkplugin.Handle) (fwkplugin.Plugin, error) {
	return NewVertexAIParser().WithName(name), nil
}

func (p *VertexAIParser) WithName(name string) *VertexAIParser {
	p.typedName.Name = name
	return p
}

var supportedVertexAIPaths = []string{
	chatCompletionsMethod,
}

// ParseRequest parses the gRPC request body and headers and returns an InferenceRequestBody.
func (p *VertexAIParser) ParseRequest(ctx context.Context, body []byte, headers map[string]string) (*fwkrh.ParseRequestResult, error) {
	logger := log.FromContext(ctx)
	path := headers[parsers.MethodPathKey]
	
	supported := false
	for _, suffix := range supportedVertexAIPaths {
		if strings.HasSuffix(path, suffix) {
			supported = true
			break
		}
	}
	
	if !supported {
		return &fwkrh.ParseRequestResult{BypassOnError: true}, fmt.Errorf("unsupported gRPC path: %s", path)
	}

	switch {
	case strings.HasSuffix(path, chatCompletionsMethod):
		parsedPayload, _, err := parsers.ParseGrpcPayload(body)
		if err != nil {
			return &fwkrh.ParseRequestResult{BypassOnError: false}, fmt.Errorf("parsing gRPC frame for ChatCompletions: %w", err)
		}

		req := &aiplatformpb.ChatCompletionsRequest{}
		if err := proto.Unmarshal(parsedPayload, req); err != nil {
			return &fwkrh.ParseRequestResult{BypassOnError: false}, fmt.Errorf("unmarshaling ChatCompletionsRequest: %w", err)
		}

		httpBody := req.GetHttpBody()
		if httpBody == nil {
			return &fwkrh.ParseRequestResult{BypassOnError: false}, fmt.Errorf("ChatCompletionsRequest has no HttpBody")
		}
		jsonBytes := httpBody.GetData()

		// Use OpenAI parser to parse the JSON payload
		openAIParser := openai.NewOpenAIParser()
		// Clone headers and set path to /v1/chat/completions to make OpenAI parser recognize it
		headersCopy := make(map[string]string)
		for k, v := range headers {
			headersCopy[k] = v
		}
		headersCopy[":path"] = "/v1/chat/completions"
		parseResult, err := openAIParser.ParseRequest(ctx, jsonBytes, headersCopy)
		if err != nil {
			return &fwkrh.ParseRequestResult{BypassOnError: parseResult != nil && parseResult.BypassOnError}, fmt.Errorf("parsing ChatCompletionsRequest: %w", err)
		}
		
		inferenceRequestBody := parseResult.Body
		inferenceRequestBody.Payload = fwkrh.PayloadProto{Message: req}
		if inferenceRequestBody.ChatCompletions != nil {
			logger.V(logutil.DEBUG).Info("Parsed ChatCompletionsRequest", "body", inferenceRequestBody.ChatCompletions.Messages)
		} else {
			logger.V(logutil.DEBUG).Info("Parsed ChatCompletionsRequest", "body", inferenceRequestBody.ChatCompletions)
		}
		return &fwkrh.ParseRequestResult{Body: inferenceRequestBody}, nil

	default:
		// Bypass if it is an unsupported path.
		return &fwkrh.ParseRequestResult{BypassOnError: true}, fmt.Errorf("unsupported gRPC path: %s", path)
	}
}

// ParseResponse parses the response body and returns a ParsedResponse
func (p *VertexAIParser) ParseResponse(ctx context.Context, body []byte, headers map[string]string, _ bool) (*fwkrh.ParsedResponse, error) {
	if len(body) == 0 {
		return nil, nil
	}

	parsedPayload, _, err := parsers.ParseGrpcPayload(body)
	if err != nil {
		return nil, fmt.Errorf("parsing gRPC frame for response: %w", err)
	}

	respMsg := &httpbody.HttpBody{}
	if err := proto.Unmarshal(parsedPayload, respMsg); err != nil {
		return nil, fmt.Errorf("unmarshaling HttpBody response: %w", err)
	}
	jsonBytes := respMsg.GetData()

	// Delegate to OpenAI parser for response as well
	openAIParser := openai.NewOpenAIParser()
	return openAIParser.ParseResponse(ctx, jsonBytes, headers, false)
}
