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

	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/scheduling/framework"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/scheduling/types"
	logutil "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/util/logging"
)

// compile-time type assertion
var _ framework.Filter = &DecisionTreeFilter{}

// DecisionTreeFilter applies current fitler, and then recursively applies next filters
// depending success or failure of the current filter.
// It can be used to construct a flow chart algorithm.
type DecisionTreeFilter struct {
	Current framework.Filter
	// NextOnSuccess filter will be applied after successfully applying the current filter.
	// The filtered results will be passed to the next filter.
	NextOnSuccess framework.Filter
	// NextOnFailure filter will be applied if current filter results in no pods.
	// The original input will be passed to the next filter.
	NextOnFailure framework.Filter
	// NextOnSuccessOrFailure is a convenience field to configure the next filter regardless of the
	// success or failure of the current filter.
	// NOTE: When using NextOnSuccessOrFailure, both nextOnSuccess and nextOnFailure SHOULD be nil.
	// However if that's not the case, nextOnSuccess and nextOnFailure will be used, instead of
	// NextOnSuccessOrFailure, in the success and failure scenarios, respectively.
	NextOnSuccessOrFailure framework.Filter
}

// Type returns the type of the filter.
func (f *DecisionTreeFilter) Type() string {
	if f == nil {
		return "nil"
	}
	return f.Current.Type()
}

// Filter filters out pods that doesn't meet the filter criteria.
func (f *DecisionTreeFilter) Filter(ctx context.Context, cycleState *types.CycleState, request *types.LLMRequest, pods []types.Pod) []types.Pod {
	loggerTrace := log.FromContext(ctx).V(logutil.TRACE)
	filteredPod := f.Current.Filter(ctx, cycleState, request, pods)

	next := f.NextOnSuccessOrFailure
	if len(filteredPod) > 0 {
		if f.NextOnSuccess == nil && f.NextOnSuccessOrFailure == nil {
			// No succeeding filters to run, return.
			return filteredPod
		}
		if f.NextOnSuccess != nil {
			next = f.NextOnSuccess
		}
		loggerTrace.Info("Filter succeeded", "filter", f.Type(), "next", next.Type(), "filteredPodCount", len(filteredPod))
		// On success, pass the filtered result to the next filter.
		return next.Filter(ctx, cycleState, request, filteredPod)
	} else {
		if f.NextOnFailure == nil && f.NextOnSuccessOrFailure == nil {
			// No succeeding filters to run, return.
			return filteredPod
		}
		if f.NextOnFailure != nil {
			next = f.NextOnFailure
		}
		loggerTrace.Info("Filter failed", "filter", f.Type(), "next", next.Type())
		// On failure, pass the initial set of pods to the next filter.
		return next.Filter(ctx, cycleState, request, pods)
	}
}
