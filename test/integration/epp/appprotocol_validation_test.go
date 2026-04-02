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

package epp

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1 "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	eppRunner "sigs.k8s.io/gateway-api-inference-extension/cmd/epp/runner"
	eppServer "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/server"
)

func TestAppProtocolValidationMismatch(t *testing.T) {
	ctx := context.Background()

	opts := eppServer.NewOptions()
	opts.ConfigText = `
apiVersion: inference.networking.x-k8s.io/v1alpha1
kind: EndpointPickerConfig
plugins:
  - type: vllmgrpc-parser
parser:
  pluginRef: vllmgrpc-parser
`
	h := NewTestHarness(t, ctx, WithConfigText(opts.ConfigText))

	// Create an InferencePool with AppProtocolH2C which is compatible with vllmgrpc-parser
	pool := &v1.InferencePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testPoolName,
			Namespace: h.Namespace,
		},
		Spec: v1.InferencePoolSpec{
			AppProtocol: v1.AppProtocolH2C,
			Selector: v1.LabelSelector{
				MatchLabels: map[v1.LabelKey]v1.LabelValue{"app": "vllm"},
			},
			TargetPorts: []v1.Port{{Number: 8080}},
			EndpointPickerRef: v1.EndpointPickerRef{
				Name: "epp",
				Port: &v1.Port{Number: 8080},
			},
		},
	}
	err := k8sClient.Create(ctx, pool)
	require.NoError(t, err)

	// Wait for sync
	h.WaitForSync(0, "")
	require.True(t, h.Datastore.PoolHasSynced())

	// Update the InferencePool to an incompatible AppProtocol (http for vllmgrpc)
	err = k8sClient.Get(ctx, types.NamespacedName{Name: pool.Name, Namespace: pool.Namespace}, pool)
	require.NoError(t, err)
	pool.Spec.AppProtocol = v1.AppProtocolHTTP

	err = k8sClient.Update(ctx, pool)
	require.NoError(t, err)

	// Wait for the reconciler to run, fail validation, and clear the Datastore
	require.Eventually(t, func() bool {
		return !h.Datastore.PoolHasSynced()
	}, 10*time.Second, 100*time.Millisecond, "Timed out waiting for Datastore to be cleared after AppProtocol mismatch")
}

func TestAppProtocolValidationSuccess(t *testing.T) {
	ctx := context.Background()

	// Create an InferencePool with h2c AppProtocol
	pool := &v1.InferencePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "success-pool",
			Namespace: "default",
		},
		Spec: v1.InferencePoolSpec{
			AppProtocol: v1.AppProtocolH2C,
			Selector: v1.LabelSelector{
				MatchLabels: map[v1.LabelKey]v1.LabelValue{"app": "vllm"},
			},
			TargetPorts: []v1.Port{
				{Number: 8080},
			},
			EndpointPickerRef: v1.EndpointPickerRef{
				Name: "epp",
				Port: &v1.Port{Number: 8080},
			},
		},
	}
	err := k8sClient.Create(ctx, pool)
	require.NoError(t, err)
	defer func() {
		_ = k8sClient.Delete(ctx, pool)
	}()

	opts := eppServer.NewOptions()
	opts.PoolName = pool.Name
	opts.PoolNamespace = pool.Namespace
	opts.PoolGroup = v1.GroupName
	opts.ConfigText = `
apiVersion: inference.networking.x-k8s.io/v1alpha1
kind: EndpointPickerConfig
plugins:
  - type: vllmgrpc-parser
parser:
  pluginRef: vllmgrpc-parser
`

	_, _, err = eppRunner.NewTestRunnerSetup(ctx, testEnv.Config, opts, nil, nil)

	require.NoError(t, err)
}
