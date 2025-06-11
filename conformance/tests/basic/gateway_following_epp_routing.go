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

package basic

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types" // For standard condition types
	"sigs.k8s.io/gateway-api/conformance/utils/suite"
	"sigs.k8s.io/gateway-api/pkg/features" // For standard feature names

	// Import the tests package to append to ConformanceTests
	"sigs.k8s.io/gateway-api-inference-extension/conformance/tests"
	"sigs.k8s.io/gateway-api-inference-extension/conformance/utils/config"
	k8sutils "sigs.k8s.io/gateway-api-inference-extension/conformance/utils/kubernetes"
	trafficutils "sigs.k8s.io/gateway-api-inference-extension/conformance/utils/traffic"
)

func init() {
	// Register the GatewayFollowingEPPRouting test case with the conformance suite.
	// This ensures it will be discovered and run by the test runner.
	tests.ConformanceTests = append(tests.ConformanceTests, GatewayFollowingEPPRouting)
}

// GatewayFollowingEPPRouting defines the test case for verifying gateway should send traffic to an endpoint in the list returned by EPP.
var GatewayFollowingEPPRouting = suite.ConformanceTest{
	ShortName:   "GatewayFollowingEPPRouting",
	Description: "Inference gateway should send traffic to an endpoint in the list returned by EPP",
	Manifests:   []string{"tests/basic/gateway_following_epp_routing.yaml"},
	Features: []features.FeatureName{
		features.FeatureName("SupportInferencePool"),
		features.SupportGateway,
	},
	Test: func(t *testing.T, s *suite.ConformanceTestSuite) {
		const (
			appBackendNamespace = "gateway-conformance-app-backend"
			infraNamespace      = "gateway-conformance-infra"
			hostname            = "primary.example.com"
			path                = "/primary-gateway-test"
			backendName         = "infra-backend-deployment"
		)

		httpRouteNN := types.NamespacedName{Name: "httproute-for-primary-gw", Namespace: appBackendNamespace}
		gatewayNN := types.NamespacedName{Name: "conformance-gateway", Namespace: infraNamespace}
		poolNN := types.NamespacedName{Name: "normal-gateway-pool", Namespace: appBackendNamespace}
		backendPodLabels := map[string]string{"app": "infra-backend"}

		k8sutils.HTTPRouteMustBeAcceptedAndResolved(t, s.Client, s.TimeoutConfig, httpRouteNN, gatewayNN)
		k8sutils.InferencePoolMustBeAcceptedByParent(t, s.Client, poolNN)
		gwAddr := k8sutils.GetGatewayEndpoint(t, s.Client, s.TimeoutConfig, gatewayNN)

		backendPodIP, err := k8sutils.GetOnePodIPWithLabel(t, s.Client, appBackendNamespace, backendPodLabels)
		require.NoError(t, err, "Failed to get backend Pod IP address")

		inferenceTimeoutConfig := config.DefaultInferenceExtensionTimeoutConfig()
		// TODO: replace this with a poll and check.
		t.Log("Waiting for the httpRoute and inferecePool ready to serve traffic.")
		time.Sleep(inferenceTimeoutConfig.WaitForHttpRouteAndInferencePoolReadyTimeout)

		correctRequestBody := `{
            "model": "conformance-fake-model",
			"prompt": "Write as if you were a critic: San Francisco"
        }`

		t.Run("Gateway should send traffic to a valid endpoint specified by EPP", func(t *testing.T) {
			t.Logf("Sending request to %s with EPP header routing to valid IP %s", gwAddr, backendPodIP)
			eppHeader := map[string]string{"test-epp-endpoint-selection": backendPodIP}

			trafficutils.MakeRequestAndExpectSuccessV2(
				t,
				s.RoundTripper,
				s.TimeoutConfig,
				gwAddr,
				trafficutils.Request{
					Host:      hostname,
					Path:      path,
					Headers:   eppHeader,
					Method:    http.MethodPost,
					Body:      correctRequestBody,
					Backend:   backendName,
					Namespace: appBackendNamespace,
				},
			)
		})

		t.Run("Gateway should send traffic specified by EPP even an invalidIP and should get response with error code 429", func(t *testing.T) {
			invalidIP := "256.256.256.256" // An IP that cannot be a real endpoint
			t.Logf("Sending request to %s with EPP header routing to invalid IP %s", gwAddr, invalidIP)
			eppHeader := map[string]string{"test-epp-endpoint-selection": invalidIP}

			trafficutils.MakeRequestAndExpectEventuallyConsistentResponse(
				t,
				s.RoundTripper,
				s.TimeoutConfig,
				gwAddr,
				trafficutils.Request{
					Host:      hostname,
					Path:      path,
					Headers:   eppHeader,
					Method:    http.MethodPost,
					Body:      correctRequestBody,
					Namespace: appBackendNamespace,

					ExpectedStatusCode: http.StatusTooManyRequests,
				},
			)
		})

		t.Run("Gateway should reject request that is missing the model name and return 400 response", func(t *testing.T) {
			requestBodyWithoutModel := `{"prompt": "Write as if you were a critic: San Francisco"}`
			eppHeader := map[string]string{"test-epp-endpoint-selection": backendPodIP}
			t.Logf("Sending request to %s with a malformed body (missing model)", gwAddr)

			trafficutils.MakeRequestAndExpectEventuallyConsistentResponse(
				t,
				s.RoundTripper,
				s.TimeoutConfig,
				gwAddr,
				trafficutils.Request{
					Host:      hostname,
					Path:      path,
					Headers:   eppHeader,
					Method:    http.MethodPost,
					Body:      requestBodyWithoutModel,
					Namespace: appBackendNamespace,

					ExpectedStatusCode: http.StatusBadRequest,
				},
			)
		})

		t.Run("Gateway should reject request that is with a nonexist model name and return 404 response", func(t *testing.T) {
			requestBodyNonExistModel := `{
            	"model": "non-exist-model",
				"prompt": "Write as if you were a critic: San Francisco"
        	}`
			eppHeader := map[string]string{"test-epp-endpoint-selection": backendPodIP}
			t.Logf("Sending request to %s with a malformed body (nonexist model)", gwAddr)

			trafficutils.MakeRequestAndExpectEventuallyConsistentResponse(
				t,
				s.RoundTripper,
				s.TimeoutConfig,
				gwAddr,
				trafficutils.Request{
					Host:      hostname,
					Path:      path,
					Headers:   eppHeader,
					Method:    http.MethodPost,
					Body:      requestBodyNonExistModel,
					Namespace: appBackendNamespace,

					ExpectedStatusCode: http.StatusNotFound,
				},
			)

		})
	},
}
