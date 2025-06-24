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
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/conformance/utils/suite"
	"sigs.k8s.io/gateway-api/pkg/features"

	"sigs.k8s.io/gateway-api-inference-extension/conformance/tests"
	"sigs.k8s.io/gateway-api-inference-extension/conformance/utils/config"
	k8sutils "sigs.k8s.io/gateway-api-inference-extension/conformance/utils/kubernetes"
	trafficutils "sigs.k8s.io/gateway-api-inference-extension/conformance/utils/traffic"
)

func init() {
	tests.ConformanceTests = append(tests.ConformanceTests, InferencePoolParentStatus)
}

var InferencePoolParentStatus = suite.ConformanceTest{
	ShortName:   "InferencePoolResolvedRefsCondition",
	Description: "Verify that an InferencePool correctly updates its parent-specific status (e.g., Accepted condition) when referenced by HTTPRoutes attached to shared Gateways, and clears parent statuses when no longer referenced.",
	Manifests:   []string{"tests/basic/inferencepool_resolvedrefs_condition.yaml"},
	Features: []features.FeatureName{
		features.FeatureName("SupportInferencePool"),
		features.SupportGateway,
	},
	Test: func(t *testing.T, s *suite.ConformanceTestSuite) {
		const (
			appBackendNamespace        = "gateway-conformance-app-backend"
			infraNamespace             = "gateway-conformance-infra"
			poolName                   = "primary-inference-pool"
			sharedPrimaryGatewayName   = "conformance-primary-gateway"
			sharedSecondaryGatewayName = "conformance-secondary-gateway"
			httpRoutePrimaryName       = "httproute-for-primary-gw"
			httpRouteSecondaryName     = "httproute-for-secondary-gw"
			hostnamePrimaryGw          = "primary.example.com"
			pathPrimaryGw              = "/primary-gateway-test"
			hostnameSecondaryGw        = "secondary.example.com"
			pathSecondaryGw            = "/secondary-gateway-test"
			backendServicePodName      = "primary-inference-model-server-deployment"
		)

		poolNN := types.NamespacedName{Name: poolName, Namespace: appBackendNamespace}
		httpRoutePrimaryNN := types.NamespacedName{Name: httpRoutePrimaryName, Namespace: appBackendNamespace}
		httpRouteSecondaryNN := types.NamespacedName{Name: httpRouteSecondaryName, Namespace: appBackendNamespace}
		gatewayPrimaryNN := types.NamespacedName{Name: sharedPrimaryGatewayName, Namespace: infraNamespace}
		gatewaySecondaryNN := types.NamespacedName{Name: sharedSecondaryGatewayName, Namespace: infraNamespace}

		inferenceTimeoutConfig := config.DefaultInferenceExtensionTimeoutConfig()

		k8sutils.HTTPRouteMustBeAcceptedAndResolved(t, s.Client, s.TimeoutConfig, httpRoutePrimaryNN, gatewayPrimaryNN)
		k8sutils.HTTPRouteMustBeAcceptedAndResolved(t, s.Client, s.TimeoutConfig, httpRouteSecondaryNN, gatewaySecondaryNN)

		gwPrimaryAddr := k8sutils.GetGatewayEndpoint(t, s.Client, s.TimeoutConfig, gatewayPrimaryNN)
		gwSecondaryAddr := k8sutils.GetGatewayEndpoint(t, s.Client, s.TimeoutConfig, gatewaySecondaryNN)

		t.Run("InferencePool should show Accepted:True by parents and be routable via multiple HTTPRoutes", func(t *testing.T) {
			k8sutils.InferencePoolMustBeAcceptedByParentGateway(t, s.Client, poolNN, gatewayPrimaryNN)
			t.Logf("InferencePool %s has parent status Accepted:True as expected with two references.", poolNN.String())

			trafficutils.MakeRequestAndExpectSuccess(
				t,
				s.RoundTripper,
				s.TimeoutConfig,
				gwPrimaryAddr,
				trafficutils.Request{
					Host:      hostnamePrimaryGw,
					Path:      pathPrimaryGw,
					Backend:   backendServicePodName,
					Namespace: appBackendNamespace,
				},
			)

			trafficutils.MakeRequestAndExpectSuccess(
				t,
				s.RoundTripper,
				s.TimeoutConfig,
				gwSecondaryAddr,
				trafficutils.Request{
					Host:      hostnameSecondaryGw,
					Path:      pathSecondaryGw,
					Backend:   backendServicePodName,
					Namespace: appBackendNamespace,
				},
			)
		})

		t.Run("Delete httproute-for-primary-gw and verify InferencePool status and routing via secondary gw", func(t *testing.T) {
			httpRoutePrimary := &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{Name: httpRoutePrimaryNN.Name, Namespace: httpRoutePrimaryNN.Namespace},
			}
			t.Logf("Deleting HTTPRoute %s", httpRoutePrimaryNN.String())
			require.NoError(t, s.Client.Delete(context.TODO(), httpRoutePrimary), "failed to delete httproute-for-primary-gw")

			t.Logf("Waiting for %v for Gateway conditions to update after deleting HTTPRoute %s", inferenceTimeoutConfig.HTTPRouteDeletionReconciliationTimeout, httpRoutePrimaryNN.String())
			time.Sleep(inferenceTimeoutConfig.HTTPRouteDeletionReconciliationTimeout)

			k8sutils.InferencePoolMustBeAcceptedByParentGateway(t, s.Client, poolNN, gatewaySecondaryNN)
			t.Logf("InferencePool %s still has parent status Accepted:True as expected with one reference remaining.", poolNN.String())

			trafficutils.MakeRequestAndExpectSuccess(
				t,
				s.RoundTripper,
				s.TimeoutConfig,
				gwSecondaryAddr,
				trafficutils.Request{
					Host:      hostnameSecondaryGw,
					Path:      pathSecondaryGw,
					Backend:   backendServicePodName,
					Namespace: appBackendNamespace,
				},
			)

			trafficutils.MakeRequestAndExpectEventuallyConsistentResponse(
				t,
				s.RoundTripper,
				s.TimeoutConfig,
				gwPrimaryAddr,
				trafficutils.Request{
					Host:               hostnamePrimaryGw,
					Path:               pathPrimaryGw,
					ExpectedStatusCode: http.StatusNotFound,
				},
			)
		})

		t.Run("Delete httproute-for-secondary-gw and verify InferencePool has no parent statuses and is not routable", func(t *testing.T) {
			httpRouteSecondary := &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{Name: httpRouteSecondaryNN.Name, Namespace: httpRouteSecondaryNN.Namespace},
			}
			t.Logf("Deleting HTTPRoute %s", httpRouteSecondaryNN.String())
			require.NoError(t, s.Client.Delete(context.TODO(), httpRouteSecondary), "failed to delete httproute-for-secondary-gw")

			k8sutils.InferencePoolMustHaveNoParents(t, s.Client, poolNN)
			t.Logf("InferencePool %s correctly shows no parent statuses, indicating it's no longer referenced.", poolNN.String())

			trafficutils.MakeRequestAndExpectEventuallyConsistentResponse(
				t,
				s.RoundTripper,
				s.TimeoutConfig,
				gwSecondaryAddr,
				trafficutils.Request{
					Host:               hostnameSecondaryGw,
					Path:               pathSecondaryGw,
					ExpectedStatusCode: http.StatusNotFound,
				},
			)
		})

		t.Logf("InferencePoolResolvedRefsCondition test completed.")
	},
}
