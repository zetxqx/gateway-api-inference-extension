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

package backend

import "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/requestcontrol"

// Parser defines the interface for parsing requests and responses.
type Parser interface {
	// ParseRequest parses the request body and headers and returns a map representation.
	ParseRequest(body []byte, headers map[string]string) (map[string]any, error)

	// ParseResponse parses the response body and returns a map representation and usage statistics.
	ParseResponse(body []byte) (map[string]any, requestcontrol.Usage, error)

	// ParseStreamResponse parses a chunk of the streaming response and returns usage statistics and a boolean indicating if the stream is complete.
	ParseStreamResponse(chunk []byte) (map[string]any, requestcontrol.Usage, bool, error)
}
