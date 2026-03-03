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

package requesthandle

import (
	fwkplugin "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/plugin"

	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/requestcontrol"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/scheduling"
)

// Parser defines the interface for parsing payload(requests and responses).
type Parser interface {
	fwkplugin.Plugin
	// ParseRequest parses the request body and headers and returns a map representation.
	ParseRequest(body []byte, headers map[string]string) (*scheduling.LLMRequestBody, error)

	// ParseResponse parses the response payload.
	// In the streaming case (isStreaming is true), this method is invoked multiple times,
	// once per chunk sent by the model server, where 'body' represents an individual chunk.
	// In the non-streaming case, this method is invoked exactly once, where 'body' represents
	// the complete response.
	ParseResponse(body []byte, isStreaming bool) (*ParsedResponse, error)
}

type ParsedResponse struct {
	// Usage is only populate when the raw response has usage.
	Usage *requestcontrol.Usage
}
