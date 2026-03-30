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

package noop

import (
	"context"
	"encoding/json"

	fwkplugin "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/plugin"
	fwkrh "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/requesthandling"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/scheduling"
)

const (
	NoopParserType = "noop-parser"
)

// compile-time type validation
var _ fwkrh.Parser = &NoopParser{}

// NoopParser implements the fwkrh.Parser interface and does nothing.
type NoopParser struct {
	typedName fwkplugin.TypedName
}

// NewNoopParser creates a new NoopParser.
func NewNoopParser() *NoopParser {
	return &NoopParser{
		typedName: fwkplugin.TypedName{
			Type: NoopParserType,
			Name: NoopParserType,
		},
	}
}

// TypedName returns the type and name tuple of this plugin instance.
func (p *NoopParser) TypedName() fwkplugin.TypedName {
	return p.typedName
}

func NoopParserPluginFactory(name string, _ json.RawMessage, _ fwkplugin.Handle) (fwkplugin.Plugin, error) {
	return NewNoopParser().WithName(name), nil
}

func (p *NoopParser) WithName(name string) *NoopParser {
	p.typedName.Name = name
	return p
}

// ParseRequest converts the request to RawPayload.
func (p *NoopParser) ParseRequest(ctx context.Context, body []byte, headers map[string]string) (*scheduling.LLMRequestBody, error) {
	return &scheduling.LLMRequestBody{
		Payload: scheduling.RawPayload(body),
	}, nil
}

// ParseResponse does nothing and returns nil.
func (p *NoopParser) ParseResponse(ctx context.Context, body []byte, headers map[string]string, isEnd bool) (*fwkrh.ParsedResponse, error) {
	return nil, nil
}
