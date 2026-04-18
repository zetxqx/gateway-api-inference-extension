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

package loraaffinity

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	fwkdl "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/datalayer"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/plugin"
	fwksched "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/scheduling"
)

func TestLoraAffinityScorer(t *testing.T) {
	tests := []struct {
		name                   string
		request                *fwksched.InferenceRequest
		endpoints              []fwksched.Endpoint
		expectedScoresEndpoint map[string]float64 // Map of endpoint name to expected score
	}{
		{
			name:    "Target model is active",
			request: &fwksched.InferenceRequest{TargetModel: "active-model-1"},
			endpoints: []fwksched.Endpoint{
				fwksched.NewEndpoint(
					&fwkdl.EndpointMetadata{Key: plugin.NewEndpointKey("pod1", "default", 8000)},
					&fwkdl.Metrics{
						ActiveModels:    map[string]int{"active-model-1": 1},
						WaitingModels:   map[string]int{},
						MaxActiveModels: 5,
					}, nil),
			},
			expectedScoresEndpoint: map[string]float64{
				"default/pod1:8000": 1.0,
			},
		},
		{
			name:    "Target model is waiting",
			request: &fwksched.InferenceRequest{TargetModel: "active-model-1"},
			endpoints: []fwksched.Endpoint{
				fwksched.NewEndpoint(
					&fwkdl.EndpointMetadata{Key: plugin.NewEndpointKey("pod1", "default", 8000)},
					&fwkdl.Metrics{
						ActiveModels:    map[string]int{"active-model-2": 2},
						WaitingModels:   map[string]int{"active-model-1": 1},
						MaxActiveModels: 2,
					}, nil),
			},
			expectedScoresEndpoint: map[string]float64{
				"default/pod1:8000": 0.6,
			},
		},
		{
			name:    "Endpoints have no space for new model",
			request: &fwksched.InferenceRequest{TargetModel: "active-model-1"},
			endpoints: []fwksched.Endpoint{
				fwksched.NewEndpoint(
					&fwkdl.EndpointMetadata{Key: plugin.NewEndpointKey("pod1", "default", 8000)},
					&fwkdl.Metrics{
						ActiveModels:    map[string]int{"active-model-2": 2},
						WaitingModels:   map[string]int{"active-model-3": 1},
						MaxActiveModels: 2,
					}, nil),
				fwksched.NewEndpoint(
					&fwkdl.EndpointMetadata{Key: plugin.NewEndpointKey("pod2", "default", 8000)},
					&fwkdl.Metrics{
						ActiveModels:    map[string]int{},
						WaitingModels:   map[string]int{},
						MaxActiveModels: 0,
					}, nil),
			},
			expectedScoresEndpoint: map[string]float64{
				"default/pod1:8000": 0.0,
				"default/pod2:8000": 0.0,
			},
		},
		{
			name:    "Multipleendpoints with mixed active and waiting models",
			request: &fwksched.InferenceRequest{TargetModel: "active-model-1"},
			endpoints: []fwksched.Endpoint{
				fwksched.NewEndpoint(
					&fwkdl.EndpointMetadata{Key: plugin.NewEndpointKey("pod1", "default", 8000)},
					&fwkdl.Metrics{
						ActiveModels:    map[string]int{"active-model-1": 1},
						WaitingModels:   map[string]int{},
						MaxActiveModels: 5,
					}, nil),
				fwksched.NewEndpoint(
					&fwkdl.EndpointMetadata{Key: plugin.NewEndpointKey("pod2", "default", 8000)},
					&fwkdl.Metrics{
						ActiveModels:    map[string]int{"active-model-2": 4},
						WaitingModels:   map[string]int{"active-model-1": 1},
						MaxActiveModels: 5,
					}, nil),
				fwksched.NewEndpoint(
					&fwkdl.EndpointMetadata{Key: plugin.NewEndpointKey("pod3", "default", 8000)},
					&fwkdl.Metrics{
						ActiveModels:    map[string]int{"active-model-2": 1},
						WaitingModels:   map[string]int{},
						MaxActiveModels: 2,
					}, nil),
				fwksched.NewEndpoint(
					&fwkdl.EndpointMetadata{Key: plugin.NewEndpointKey("pod4", "default", 8000)},
					&fwkdl.Metrics{
						ActiveModels:    map[string]int{"active-model-3": 1},
						WaitingModels:   map[string]int{"active-model-1": 1},
						MaxActiveModels: 2,
					}, nil),
				fwksched.NewEndpoint(
					&fwkdl.EndpointMetadata{Key: plugin.NewEndpointKey("pod5", "default", 8000)},
					&fwkdl.Metrics{
						ActiveModels:    map[string]int{"active-model-4": 1, "active-model-5": 1},
						WaitingModels:   map[string]int{},
						MaxActiveModels: 2,
					}, nil),
			},
			expectedScoresEndpoint: map[string]float64{
				"default/pod1:8000": 1.0,
				"default/pod2:8000": 0.8,
				"default/pod3:8000": 0.8,
				"default/pod4:8000": 0.6,
				"default/pod5:8000": 0.0,
			},
		},
		{
			name:                   "Empty pods slice",
			request:                &fwksched.InferenceRequest{TargetModel: "modelA"},
			endpoints:              []fwksched.Endpoint{},
			expectedScoresEndpoint: map[string]float64{}, // No pods, no scores
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scorer := &LoraAffinityScorer{}
			scores := scorer.Score(context.Background(), fwksched.NewCycleState(), test.request, test.endpoints)

			for _, endpoint := range test.endpoints {
				endpointKey := endpoint.GetMetadata().GetKey().String()
				expectedScore, ok := test.expectedScoresEndpoint[endpointKey]
				if !ok {
					t.Fatalf("Expected score not found for endpoint %s in test %s", endpointKey, test.name)
				}
				assert.InDelta(t, expectedScore, scores[endpoint], 0.0001, "Endpoint %s should have score %f", endpointKey, expectedScore)
			}
			assert.Len(t, scores, len(test.expectedScoresEndpoint), "Number of scored endpoints should match expected")
		})
	}
}
