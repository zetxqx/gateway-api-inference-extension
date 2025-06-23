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

package requestcontrol

import (
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/plugins"
)

// NewConfig creates a new Config object and returns its pointer.
func NewConfig() *Config {
	return &Config{
		preRequestPlugins:   []PreRequest{},
		postResponsePlugins: []PostResponse{},
	}
}

// Config provides a configuration for the requestcontrol plugins.
type Config struct {
	preRequestPlugins   []PreRequest
	postResponsePlugins []PostResponse
}

// WithPreRequestPlugins sets the given plugins as the PreRequest plugins.
// If the Config has PreRequest plugins already, this call replaces the existing plugins with the given ones.
func (c *Config) WithPreRequestPlugins(plugins ...PreRequest) *Config {
	c.preRequestPlugins = plugins
	return c
}

// WithPostResponsePlugins sets the given plugins as the PostResponse plugins.
// If the Config has PostResponse plugins already, this call replaces the existing plugins with the given ones.
func (c *Config) WithPostResponsePlugins(plugins ...PostResponse) *Config {
	c.postResponsePlugins = plugins
	return c
}

func (c *Config) AddPlugins(pluginObjects ...plugins.Plugin) {
	for _, plugin := range pluginObjects {
		if preRequestPlugin, ok := plugin.(PreRequest); ok {
			c.preRequestPlugins = append(c.preRequestPlugins, preRequestPlugin)
		}
		if postResponsePlugin, ok := plugin.(PostResponse); ok {
			c.postResponsePlugins = append(c.postResponsePlugins, postResponsePlugin)
		}
	}
}

func LoadRequestControlConfig(instantiatedPlugins map[string]plugins.Plugin) *Config {
	config := NewConfig()
	for _, plugin := range instantiatedPlugins {
		config.AddPlugins(plugin)
	}

	return config
}
