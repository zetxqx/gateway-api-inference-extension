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

package picker

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"

	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/plugins"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/scheduling/framework"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/scheduling/types"
	logutil "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/util/logging"
)

const (
	RandomPickerType = "random"
)

// compile-time type validation
var _ framework.Picker = &RandomPicker{}

// RandomPickerFactory defines the factory function for RandomPicker.
func RandomPickerFactory(name string, _ json.RawMessage, _ plugins.Handle) (plugins.Plugin, error) {
	return NewRandomPicker(), nil
}

// NewRandomPicker initializes a new RandomPicker and returns its pointer.
func NewRandomPicker() *RandomPicker {
	return &RandomPicker{}
}

// RandomPicker picks a random pod from the list of candidates.
type RandomPicker struct{}

// Type returns the type of the picker.
func (p *RandomPicker) Type() string {
	return RandomPickerType
}

// Pick selects a random pod from the list of candidates.
func (p *RandomPicker) Pick(ctx context.Context, _ *types.CycleState, scoredPods []*types.ScoredPod) *types.ProfileRunResult {
	log.FromContext(ctx).V(logutil.DEBUG).Info(fmt.Sprintf("Selecting a random pod from %d candidates: %+v", len(scoredPods), scoredPods))
	i := rand.Intn(len(scoredPods))
	return &types.ProfileRunResult{TargetPod: scoredPods[i]}
}
