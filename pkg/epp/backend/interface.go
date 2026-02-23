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

// BackendRequest defines the interface for a parsed request that can be inspected, modified, and re-marshaled.
type BackendRequest interface {
	// Get returns the value of a specific field.
	Get(key string) (any, bool)
	// Set updates a field in the request.
	Set(key string, value any) error
	// Marshal serializes the request back to bytes (JSON or Proto).
	Marshal() ([]byte, error)
	// ToMap returns a map representation of the request.
	ToMap() map[string]any
}

// Parser defines the interface for parsing requests and responses.
type Parser interface {
	// ParseRequest parses the request body and headers and returns a BackendRequest object.
	ParseRequest(body []byte, headers map[string]string) (BackendRequest, error)

	// ParseResponse parses the response body and returns a map representation and usage statistics.
	ParseResponse(body []byte) (map[string]any, requestcontrol.Usage, error)

	// ParseStreamResponse parses a chunk of the streaming response and returns usage statistics and a boolean indicating if the stream is complete.
	ParseStreamResponse(chunk []byte) (map[string]any, requestcontrol.Usage, bool, error)
}
