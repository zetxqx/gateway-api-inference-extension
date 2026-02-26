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

package payloadprocess

import (
	"errors"

	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/payloadprocess"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/plugin"

	fwk "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/payloadprocess"
)

// Config holds the configuration for the SaturationDetector.
type Config struct {
	parser payloadprocess.Parser
}

func NewParser(pluginObjects []plugin.Plugin) (payloadprocess.Parser, error) {
	parserPlugins := []fwk.Parser{}
	for _, plugin := range pluginObjects {
		if parser, ok := plugin.(fwk.Parser); ok {
			parserPlugins = append(parserPlugins, parser)
		}
	}
	if len(parserPlugins) > 1 {
		return nil, errors.New("Error more than one parser plugin")
	}
	if len(parserPlugins) == 1 {
		return parserPlugins[0], nil
	}
	return nil, nil
}
