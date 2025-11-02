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

package tests

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	gwhttp "sigs.k8s.io/gateway-api/conformance/utils/http"
	"sigs.k8s.io/gateway-api/conformance/utils/suite"
	"sigs.k8s.io/gateway-api/pkg/features"

	"sigs.k8s.io/gateway-api-inference-extension/conformance/resources"
	k8sutils "sigs.k8s.io/gateway-api-inference-extension/conformance/utils/kubernetes"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/scheduling/framework/plugins/test"
)

func init() {
	ConformanceTests = append(ConformanceTests, GatewayValidatesEppEndpoints)
}

var GatewayValidatesEppEndpoints = suite.ConformanceTest{
	ShortName:   "GatewayValidatesEppEndpoints",
	Description: "A Gateway should validate that endpoints returned by an EPP are part of a known admin configuration (e.g. same namespace).",
	Manifests:   []string{"tests/gateway_validates_epp_endpoints.yaml"},
	Features: []features.FeatureName{
		features.FeatureName("SupportInferencePool"),
		features.SupportGateway,
	},
	Test: func(t *testing.T, s *suite.ConformanceTestSuite) {
		const (
			hostname            = "validates-epp-endpoints.example.com"
			path                = "/test"
			altBackendNamespace = "inference-conformance-app-backend-alt"
			altBackendAppLabel  = "alt-namespace-backend"
			requestBody         = `{"model": "conformance-fake-model", "prompt": "Write as if you were a critic: San Francisco"}`
		)

		httpRouteNN := types.NamespacedName{Name: "httproute-validates-epp-endpoints", Namespace: resources.AppBackendNamespace}
		gatewayNN := resources.PrimaryGatewayNN
		poolNN := resources.PrimaryInferencePoolNN

		t.Log("Verifying HTTPRoute and InferencePool are accepted and the Gateway has an address.")
		k8sutils.HTTPRouteMustBeAcceptedAndResolved(t, s.Client, s.TimeoutConfig, httpRouteNN, gatewayNN)
		k8sutils.InferencePoolMustBeAcceptedByParent(t, s.Client, poolNN, gatewayNN)
		gwAddr := k8sutils.GetGatewayEndpoint(t, s.Client, s.TimeoutConfig, gatewayNN)

		t.Log("Fetching backend pod in the same namespace")
		sameNamespacePods, err := k8sutils.GetPodsWithLabel(t, s.Client, resources.AppBackendNamespace, map[string]string{"app": resources.PrimaryModelServerAppLabel}, s.TimeoutConfig)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(sameNamespacePods), 1, "Expected at least one backend pod in the same namespace")
		sameNamespacePodIP := sameNamespacePods[0].Status.PodIP
		sameNamespacePodName := sameNamespacePods[0].Name

		t.Log("Fetching backend pod in the different namespace")
		differentNamespacePods, err := k8sutils.GetPodsWithLabel(t, s.Client, altBackendNamespace, map[string]string{"app": altBackendAppLabel}, s.TimeoutConfig)
		require.NoError(t, err)
		require.Len(t, differentNamespacePods, 1, "Expected to find 1 backend pod in the different namespace")
		differentNamespacePodIP := differentNamespacePods[0].Status.PodIP

		t.Run("Request to an endpoint in the same namespace should be accepted", func(t *testing.T) {
			t.Logf("Sending request with EPP header to select pod IP %s in the same namespace", sameNamespacePodIP)
			gwhttp.MakeRequestAndExpectEventuallyConsistentResponse(
				t,
				s.RoundTripper,
				s.TimeoutConfig,
				gwAddr,
				gwhttp.ExpectedResponse{
					Request: gwhttp.Request{
						Host:   hostname,
						Path:   path,
						Method: http.MethodPost,
						Body:   requestBody,
						Headers: map[string]string{
							test.HeaderTestEppEndPointSelectionKey: sameNamespacePodIP,
						},
					},
					Response: gwhttp.Response{
						StatusCodes: []int{http.StatusOK},
					},
					Backend:   sameNamespacePodName,
					Namespace: resources.AppBackendNamespace,
				},
			)
		})

		t.Run("Request to an endpoint in a different namespace should be denied", func(t *testing.T) {
			t.Logf("Sending request with EPP header to select pod IP %s in a different namespace", differentNamespacePodIP)
			gwhttp.MakeRequestAndExpectEventuallyConsistentResponse(
				t,
				s.RoundTripper,
				s.TimeoutConfig,
				gwAddr,
				gwhttp.ExpectedResponse{
					Request: gwhttp.Request{
						Host:   hostname,
						Path:   path,
						Method: http.MethodPost,
						Body:   requestBody,
						Headers: map[string]string{
							test.HeaderTestEppEndPointSelectionKey: differentNamespacePodIP,
						},
					},
					Response: gwhttp.Response{
						// Expecting a server-side error as the gateway should reject the endpoint.
						StatusCodes: []int{http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable},
					},
				},
			)
		})
	},
}
