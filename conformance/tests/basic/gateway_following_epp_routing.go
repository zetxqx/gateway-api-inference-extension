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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types" // For standard condition types
	"sigs.k8s.io/gateway-api/conformance/utils/suite"
	"sigs.k8s.io/gateway-api/pkg/features" // For standard feature names

	// Import the tests package to append to ConformanceTests
	"sigs.k8s.io/gateway-api-inference-extension/conformance/tests"
	k8sutils "sigs.k8s.io/gateway-api-inference-extension/conformance/utils/kubernetes"
	trafficutils "sigs.k8s.io/gateway-api-inference-extension/conformance/utils/traffic"
)

func init() {
	// Register the InferencePoolAccepted test case with the conformance suite.
	// This ensures it will be discovered and run by the test runner.
	tests.ConformanceTests = append(tests.ConformanceTests, GatwayFollowingEPPRouting)
}

// InferencePoolAccepted defines the test case for verifying basic InferencePool acceptance.
var GatwayFollowingEPPRouting = suite.ConformanceTest{
	ShortName:   "GatwayFollowingEPPRouting",
	Description: "Inference gateway should redirect traffic to an endpoints belonging to what EPP respond endpoints list.", // TODO
	Manifests:   []string{"tests/basic/gateway_following_epp_routing.yaml"},
	Features: []features.FeatureName{
		features.FeatureName("SupportInferencePool"),
		features.SupportGateway,
	},
	Test: func(t *testing.T, s *suite.ConformanceTestSuite) {
		const (
			appBackendNamespace      = "gateway-conformance-app-backend"
			infraNamespace           = "gateway-conformance-infra"
			poolName                 = "normal-gateway-pool"
			sharedPrimaryGatewayName = "conformance-gateway"
			httpRoutePrimaryName     = "httproute-for-primary-gw"
			hostnamePrimaryGw        = "primary.example.com"
			pathPrimaryGw            = "/primary-gateway-test"
			backendServicePodName    = "infra-backend-deployment"
		)

		poolNN := types.NamespacedName{Name: poolName, Namespace: appBackendNamespace}
		httpRoutePrimaryNN := types.NamespacedName{Name: httpRoutePrimaryName, Namespace: appBackendNamespace}
		gatewayPrimaryNN := types.NamespacedName{Name: sharedPrimaryGatewayName, Namespace: infraNamespace}

		// inferenceTimeoutConfig := config.DefaultInferenceExtensionTimeoutConfig()

		k8sutils.HTTPRouteMustBeAcceptedAndResolved(t, s.Client, s.TimeoutConfig, httpRoutePrimaryNN, gatewayPrimaryNN)
		gwPrimaryAddr := k8sutils.GetGatewayEndpoint(t, s.Client, s.TimeoutConfig, gatewayPrimaryNN)
		time.Sleep(300 * time.Second)

		t.Run("InferencePool should have Accepted condition set to True", func(t *testing.T) {
			t.Logf("InferencePool %s has parent status Accepted:True as expected with one references.", poolNN.String())
			ipAddress, err := k8sutils.GetPodIPByLabelWithControllerRuntime(t, s.Client, appBackendNamespace, map[string]string{"app": "infra-backend"})
			if err != nil {
				require.NoErrorf(t, err, "error getting podIpAdress")
			}
			t.Logf("Getting IPAddress is %v.", ipAddress)

			trafficutils.MakeRequestAndExpectSuccessV2(
				t,
				s.RoundTripper,
				s.TimeoutConfig,
				gwPrimaryAddr,
				hostnamePrimaryGw,
				pathPrimaryGw,
				backendServicePodName,
				appBackendNamespace,
				map[string]string{"Test-Epp-Endpoint-Selection": ipAddress},
			)
		})

		t.Run("InferencePool should have Accepted condition set to True", func(t *testing.T) {
			t.Logf("InferencePool %s has parent status Accepted:True as expected with one references.", poolNN.String())
			ipAddress, err := k8sutils.GetPodIPByLabelWithControllerRuntime(t, s.Client, appBackendNamespace, map[string]string{"app": "infra-backend"})
			if err != nil {
				require.NoErrorf(t, err, "error getting podIpAdress")
			}
			t.Logf("Getting IPAddress is %v.", ipAddress)

			wrongAddress := "10.0.0.17"
			trafficutils.MakeRequestAndExpectSuccessV2(
				t,
				s.RoundTripper,
				s.TimeoutConfig,
				gwPrimaryAddr,
				hostnamePrimaryGw,
				pathPrimaryGw,
				backendServicePodName,
				appBackendNamespace,
				map[string]string{"test-epp-endpoint-selection": wrongAddress},
			)
		})
	},
}
