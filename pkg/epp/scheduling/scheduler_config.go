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

package scheduling

import (
	"errors"
	"fmt"

	"sigs.k8s.io/gateway-api-inference-extension/api/config/v1alpha1"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/plugins"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/scheduling/framework"
)

// NewSchedulerConfig creates a new SchedulerConfig object and returns its pointer.
func NewSchedulerConfig(profileHandler framework.ProfileHandler, profiles map[string]*framework.SchedulerProfile) *SchedulerConfig {
	return &SchedulerConfig{
		profileHandler: profileHandler,
		profiles:       profiles,
	}
}

// SchedulerConfig provides a configuration for the scheduler which influence routing decisions.
type SchedulerConfig struct {
	profileHandler framework.ProfileHandler
	profiles       map[string]*framework.SchedulerProfile
}

func LoadSchedulerConfig(configProfiles []v1alpha1.SchedulingProfile, references map[string]plugins.Plugin) (*SchedulerConfig, error) {

	var profiles = map[string]*framework.SchedulerProfile{}

	for _, configProfile := range configProfiles {
		profile := framework.SchedulerProfile{}

		for _, plugin := range configProfile.Plugins {
			var err error
			thePlugin := references[plugin.PluginRef]
			if theScorer, ok := thePlugin.(framework.Scorer); ok {
				if plugin.Weight == nil {
					return nil, fmt.Errorf("scorer '%s' is missing a weight", plugin.PluginRef)
				}
				thePlugin = framework.NewWeightedScorer(theScorer, *plugin.Weight)
			}
			err = profile.AddPlugins(thePlugin)
			if err != nil {
				return nil, err
			}
		}
		profiles[configProfile.Name] = &profile
	}

	var profileHandler framework.ProfileHandler
	var profileHandlerName string

	for pluginName, thePlugin := range references {
		if theProfileHandler, ok := thePlugin.(framework.ProfileHandler); ok {
			if profileHandler != nil {
				return nil, fmt.Errorf("only one profile handler is allowed. Both %s and %s are profile handlers", profileHandlerName, pluginName)
			}
			profileHandler = theProfileHandler
			profileHandlerName = pluginName
		}
	}
	if profileHandler == nil {
		return nil, errors.New("no profile handler was specified")
	}

	return NewSchedulerConfig(profileHandler, profiles), nil
}
