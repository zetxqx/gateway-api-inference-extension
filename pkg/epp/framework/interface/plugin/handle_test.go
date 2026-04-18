/*
Copyright 2026 The Kubernetes Authors.

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

package plugin

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
)

func TestEndPointKey(t *testing.T) {
	t.Run("NewEndPointKey creates correct key", func(t *testing.T) {
		key := NewEndpointKey("foo", "bar", 8080)
		expected := EndpointKey{
			namespacedName: types.NamespacedName{
				Name:      "foo",
				Namespace: "bar",
			},
			port: 8080,
		}
		if diff := cmp.Diff(expected, key, cmp.AllowUnexported(EndpointKey{})); diff != "" {
			t.Errorf("NewEndpointKey() mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("String returns correct representation", func(t *testing.T) {
		key := NewEndpointKey("foo", "bar", 8080)
		assert.Equal(t, "bar/foo:8080", key.String())
	})
}
