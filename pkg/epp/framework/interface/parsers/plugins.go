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
// 1. THE MANDATORY BASE INTERFACE
// Every plugin must implement this. It strictly handles routing, scheduling, and metrics.
type Parser interface {
	plugin.Plugin

	// Matches determines if this parser should handle the incoming request.
	Matches(reqHeaders map[string]string, reqBody []byte) *MatchResult

	// ExtractSchedulingContext translates the raw body into the internal format.
	// This is strictly for the scheduler (e.g., precise prefix cache scorers).
	ExtractSchedulingContext(ctx context.Context, headers map[string]string, reqBody []byte) (*scheduling.LLMRequest, error)

	// ExtractUsage parses the raw response body to retrieve metrics (tokens, etc.).
	ExtractUsage(ctx context.Context, headers map[string]string, respBody []byte) (*requestcontrol.Usage, error)
}

// 2. THE OPTIONAL MUTATOR INTERFACE
// Only implement this if the custom protocol supports Gateway-level body mutations
// (like BBR or InferenceModelRewrite).
type RequestMutator interface {
	// MutateRequest applies changes to the request body (e.g., updating the model name).
	// It returns the newly formatted bytes.
	MutateRequest(targetModel string, reqBody []byte) ([]byte, error)
}

// 3. THE OPTIONAL TRANSCODER INTERFACE
// Only implement this if the user wants to expose a standard OpenAI frontend
// for a backend that uses a completely different custom protocol.
type OpenAITranscoder interface {
	// TranscodeRequest converts standard OpenAI JSON into the backend's custom format.
	// Returns the custom payload (e.g., a proto.Message) and any required headers.
	TranscodeRequest(openaiBody []byte) (map[string]string, any, error)

	// TranscodeResponse converts the backend's custom response back into OpenAI JSON.
	TranscodeResponse(customRespBody []byte) ([]byte, error)
}

// MatchResult encapsulate the parser match result, this is a struct for further extensibility.
type MatchResult struct {
	Matched bool
}
