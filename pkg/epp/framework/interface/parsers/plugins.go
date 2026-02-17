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

	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/plugin"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/requestcontrol"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/scheduling"
)

const (
	// RequestParsingFeatureGate guards the pluggable request parser logic.
	RequestParsingFeatureGate = "RequestParsing"
)

// Parser defines the interface for a plugin that parses request and response bodies.
type Parser interface {
	plugin.Plugin

	// Matches returns non-nil MatchResult with matched true if this parser should handle the request based on request headers and request body.
	Matches(reqHeaders map[string]string, reqBody []byte) *MatchResult

	// ParseRequest converts the raw request body and headers to an internal LLMRequest.
	// It returns the parsed LLMRequest.
	ParseRequest(ctx context.Context, headers map[string]string, body []byte) (*scheduling.LLMRequest, error)

	// TranscodeRequest converts the internal LLMRequest back to the format expected by the backend.
	// This allows for model rewriting and other modifications to be reflected in the downstream request.
	// It returns the modified headers and body bytes.
	TranscodeRequest(ctx context.Context, request *scheduling.LLMRequest, mutations ...RequestMutation) (map[string]string, []byte, error)

	// ParseResponse converts the raw response body and headers to an internal Response representation.
	// It parses the raw body into the Response struct, potentially updating headers.
	ParseResponse(ctx context.Context, response *requestcontrol.Response, body []byte) (*requestcontrol.Response, error)

	// TranscodeResponse converts the internal Response back to the format expected by the client.
	// It returns the modified headers and body bytes.
	TranscodeResponse(ctx context.Context, response *requestcontrol.Response) (map[string]string, []byte, error)
}

type RequestMutation func(request *scheduling.LLMRequest)

type MatchResult struct {
	Matched bool
}
