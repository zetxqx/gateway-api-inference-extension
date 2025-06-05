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
	"sigs.k8s.io/gateway-api-inference-extension/conformance/scheduling/plugins/filter"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/scheduling"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/scheduling/framework"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/scheduling/framework/plugins/picker"
	profilepicker "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/scheduling/framework/plugins/profile-picker"
)

// TODO: rephrase
// NewReqHeaderBasedScheduler creates a request header based scheduler for conformance tests.
// The scheduler pick the exact endpoint from the endpoint specified in the reqeust header with field name: "test-epp-endpoint-selection".
// If the field is not specified in the request header or the value does not match any existing pods endpoints, it will return empty.
func NewReqHeaderBasedScheduler(datastore scheduling.Datastore) *scheduling.Scheduler {
	predicatableSchedulerProfile := framework.NewSchedulerProfile().WithFilters(filter.NewHeaderBasedTestingFilter()).WithPicker(picker.NewMaxScorePicker())
	return scheduling.NewSchedulerWithConfig(datastore, scheduling.NewSchedulerConfig(
		profilepicker.NewAllProfilesPicker(), map[string]*framework.SchedulerProfile{"req-header-based-profile": predicatableSchedulerProfile}))
}
