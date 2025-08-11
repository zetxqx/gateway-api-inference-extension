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

package response

import (
	"reflect"
	"testing"

	errutil "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/util/error"
)

func TestExtractContentFromResponseBody(t *testing.T) {
	testCases := []struct {
		name            string
		inputBody       map[string]any
		expectedContent string
		expectedErr     error
	}{
		{
			name: "valid response",
			inputBody: map[string]any{
				"choices": []any{
					map[string]any{
						"message": map[string]any{
							"content": "Hello! How can I help?",
						},
					},
				},
			},
			expectedContent: "Hello! How can I help?",
			expectedErr:     nil,
		},
		{
			name:      "missing choices field",
			inputBody: map[string]any{"id": "some-id"},
			expectedErr: errutil.Error{
				Code: errutil.BadRequest,
				Msg:  "'choices' field not found in response body",
			},
		},
		{
			name: "choices field is not a list",
			inputBody: map[string]any{
				"choices": "not-a-list",
			},
			expectedErr: errutil.Error{
				Code: errutil.BadRequest,
				Msg:  "'choices' field is not a list",
			},
		},
		{
			name: "choices list is empty",
			inputBody: map[string]any{
				"choices": []any{},
			},
			expectedErr: errutil.Error{
				Code: errutil.BadRequest,
				Msg:  "'choices' list is empty",
			},
		},
		{
			name: "first item in choices is not an object",
			inputBody: map[string]any{
				"choices": []any{"not-an-object"},
			},
			expectedErr: errutil.Error{
				Code: errutil.BadRequest,
				Msg:  "first item in 'choices' is not an object",
			},
		},
		{
			name: "missing message field",
			inputBody: map[string]any{
				"choices": []any{
					map[string]any{
						"finish_reason": "stop",
					},
				},
			},
			expectedErr: errutil.Error{
				Code: errutil.BadRequest,
				Msg:  "'message' field not found in the first choice",
			},
		},
		{
			name: "message field is not an object",
			inputBody: map[string]any{
				"choices": []any{
					map[string]any{
						"message": "not-an-object",
					},
				},
			},
			expectedErr: errutil.Error{
				Code: errutil.BadRequest,
				Msg:  "'message' field is not an object",
			},
		},
		{
			name: "missing content field",
			inputBody: map[string]any{
				"choices": []any{
					map[string]any{
						"message": map[string]any{
							"role": "assistant",
						},
					},
				},
			},
			expectedErr: errutil.Error{
				Code: errutil.BadRequest,
				Msg:  "'content' field not found in the message object",
			},
		},
		{
			name: "content field is not a string",
			inputBody: map[string]any{
				"choices": []any{
					map[string]any{
						"message": map[string]any{
							"content": 12345, // Not a string
						},
					},
				},
			},
			expectedErr: errutil.Error{
				Code: errutil.BadRequest,
				Msg:  "'content' field is not a string",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotContent, gotErr := ExtractContentFromResponseBody(tc.inputBody)

			if !reflect.DeepEqual(gotErr, tc.expectedErr) {
				t.Errorf("expected error '%v', but got '%v'", tc.expectedErr, gotErr)
			}

			if gotContent != tc.expectedContent {
				t.Errorf("expected content '%s', but got '%s'", tc.expectedContent, gotContent)
			}
		})
	}
}
