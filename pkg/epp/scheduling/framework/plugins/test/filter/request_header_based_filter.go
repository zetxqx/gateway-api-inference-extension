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

package filter

import (
	"context"
	"encoding/json"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/backend"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/plugins"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/scheduling/framework"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/scheduling/framework/plugins/test"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/scheduling/types"
)

const (
	// HeaderBasedTestingFilterType is the filter type that is used in plugins registry.
	HeaderBasedTestingFilterType = "header-based-testing-filter"
)

// compile-time type assertion
var _ framework.Filter = &HeaderBasedTestingFilter{}

// HeaderBasedTestingFilterFactory defines the factory function for HeaderBasedTestingFilter.
func HeaderBasedTestingFilterFactory(name string, _ json.RawMessage, _ plugins.Handle) (plugins.Plugin, error) {
	return NewHeaderBasedTestingFilter().WithName(name), nil
}

// NewHeaderBasedTestingFilter initializes a new HeaderBasedTestingFilter.
// This should only be used for testing purposes.
func NewHeaderBasedTestingFilter() *HeaderBasedTestingFilter {
	return &HeaderBasedTestingFilter{
		typedName: plugins.TypedName{Type: HeaderBasedTestingFilterType, Name: HeaderBasedTestingFilterType},
	}
}

// HeaderBasedTestingFilter filters Pods based on an address specified in the "test-epp-endpoint-selection" request header.
type HeaderBasedTestingFilter struct {
	typedName plugins.TypedName
}

// TypedName returns the type and name tuple of this plugin instance.
func (f *HeaderBasedTestingFilter) TypedName() plugins.TypedName {
	return f.typedName
}

// WithName sets the name of the filter.
func (f *HeaderBasedTestingFilter) WithName(name string) *HeaderBasedTestingFilter {
	f.typedName.Name = name
	return f
}

// Filter selects pods that match the IP addresses specified in the request header.
// For the purpose of conformance testing, if an IP address in the header does not
// correspond to any of the available pods in the InferencePool, this filter will
// construct a mock Pod object for that IP. This allows testing scenarios where the
// EPP returns an endpoint that the Gateway must validate and potentially reject
// (e.g., an endpoint in a different namespace).
func (f *HeaderBasedTestingFilter) Filter(_ context.Context, _ *types.CycleState, request *types.LLMRequest, pods []types.Pod) []types.Pod {
	headerValue, ok := request.Headers[test.HeaderTestEppEndPointSelectionKey]
	if !ok || headerValue == "" {
		return []types.Pod{}
	}

	podAddressMap := make(map[string]types.Pod, len(pods))
	for _, pod := range pods {
		podAddressMap[pod.GetPod().GetIPAddress()] = pod
	}

	endpoints := strings.Split(headerValue, ",")
	filteredPods := make([]types.Pod, 0, len(endpoints))
	for _, endpoint := range endpoints {
		trimmedEndpoint := strings.TrimSpace(endpoint)
		if pod, found := podAddressMap[trimmedEndpoint]; found {
			filteredPods = append(filteredPods, pod)
		} else {
			// The requested endpoint is not in the InferencePool.
			// Create a mock pod representation to return this arbitrary endpoint.
			// This is crucial for testing the Gateway's validation logic.
			mockV1Pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fake-pod-for-" + trimmedEndpoint,
				},
				Status: corev1.PodStatus{PodIP: trimmedEndpoint},
			}
			backendPod, err := backend.NewPod(mockV1Pod)
			if err != nil {
				continue
			}

			mockTypedPod := &types.PodMetrics{
				Pod:          backendPod,
				MetricsState: nil,
			}
			filteredPods = append(filteredPods, mockTypedPod)
		}
	}
	return filteredPods
}
