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

package openai

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"

	fwkrq "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/requestcontrol"
)

const (
	body = `
	{
		"id": "cmpl-573498d260f2423f9e42817bbba3743a",
		"object": "text_completion",
		"created": 1732563765,
		"model": "meta-llama/Llama-3.1-8B-Instruct",
		"choices": [
			{
				"index": 0,
				"text": " Chronicle\nThe San Francisco Chronicle has a new book review section, and it's a good one. The reviews are short, but they're well-written and well-informed. The Chronicle's book review section is a good place to start if you're looking for a good book review.\nThe Chronicle's book review section is a good place to start if you're looking for a good book review. The Chronicle's book review section",
				"logprobs": null,
				"finish_reason": "length",
				"stop_reason": null,
				"prompt_logprobs": null
			}
		],
		"usage": {
			"prompt_tokens": 11,
			"total_tokens": 111,
			"completion_tokens": 100
		}
	}
	`
	bodyWithCachedTokens = `
	{
		"id": "cmpl-573498d260f2423f9e42817bbba3743a",
		"object": "text_completion",
		"created": 1732563765,
		"model": "meta-llama/Llama-3.1-8B-Instruct",
		"choices": [
			{
				"index": 0,
				"text": " Chronicle\nThe San Francisco Chronicle has a new book review section, and it's a good one. The reviews are short, but they're well-written and well-informed. The Chronicle's book review section is a good place to start if you're looking for a good book review.\nThe Chronicle's book review section is a good place to start if you're looking for a good book review. The Chronicle's book review section",
				"logprobs": null,
				"finish_reason": "length",
				"stop_reason": null,
				"prompt_logprobs": null
			}
		],
		"usage": {
			"prompt_tokens": 11,
			"total_tokens": 111,
			"completion_tokens": 100,
			"prompt_token_details": {
				"cached_tokens": 10
			}
		}
	}
	`

	streamingBodyWithoutUsage = `data: {"id":"cmpl-41764c93-f9d2-4f31-be08-3ba04fa25394","object":"text_completion","created":1740002445,"model":"food-review-0","choices":[],"usage":null}
	`

	streamingBodyWithUsage = `data: {"id":"cmpl-41764c93-f9d2-4f31-be08-3ba04fa25394","object":"text_completion","created":1740002445,"model":"food-review-0","choices":[],"usage":{"prompt_tokens":7,"total_tokens":17,"completion_tokens":10}}
data: [DONE]
	`
	streamingBodyWithUsageAndCachedTokens = `data: {"id":"cmpl-41764c93-f9d2-4f31-be08-3ba04fa25394","object":"text_completion","created":1740002445,"model":"food-review-0","choices":[],"usage":{"prompt_tokens":7,"total_tokens":17,"completion_tokens":10,"prompt_token_details":{"cached_tokens":5}}}
data: [DONE]
	`
)

func TestParseRequest(t *testing.T) {
	parser := NewParser()

	// Case 1: Standard Request
	body1 := []byte(`{"model": "my-model", "prompt": "Hello"}`)
	headers1 := map[string]string{}
	req1, err := parser.ParseRequest(body1, headers1)
	if err != nil {
		t.Fatalf("ParseRequest failed: %v", err)
	}
	if val, ok := req1.Get("model"); !ok || val != "my-model" {
		t.Errorf("expected model 'my-model', got %v", val)
	}
	if val, ok := req1.Get("prompt"); !ok || val != "Hello" {
		t.Errorf("expected prompt 'Hello', got %v", val)
	}

	// Update field
	if err := req1.Set("model", "new-model"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if val, ok := req1.Get("model"); !ok || val != "new-model" {
		t.Errorf("expected model 'new-model', got %v", val)
	}

	// Marshal back
	bytes, err := req1.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	assert.JSONEq(t, `{"model": "new-model", "prompt": "Hello"}`, string(bytes))
}

func TestParseResponse(t *testing.T) {
	tests := []struct {
		name    string
		body    []byte
		want    fwkrq.Usage
		wantErr bool
	}{
		{
			name: "success",
			body: []byte(body),
			want: fwkrq.Usage{
				PromptTokens:     11,
				TotalTokens:      111,
				CompletionTokens: 100,
			},
		},
		{
			name: "success with cached tokens",
			body: []byte(bodyWithCachedTokens),
			want: fwkrq.Usage{
				PromptTokens:     11,
				TotalTokens:      111,
				CompletionTokens: 100,
				PromptTokenDetails: &fwkrq.PromptTokenDetails{
					CachedTokens: 10,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parser := NewParser()
			_, usage, err := parser.ParseResponse(test.body)
			if err != nil {
				if !test.wantErr {
					t.Fatalf("ParseResponse returned unexpected error: %v, want %v", err, test.wantErr)
				}
				return
			}

			if diff := cmp.Diff(test.want, usage); diff != "" {
				t.Errorf("ParseResponse returned unexpected usage, diff(-want, +got): %v", diff)
			}
		})
	}
}

func TestParseStreamResponse(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		want    fwkrq.Usage
		wantErr bool
	}{
		{
			name: "streaming request without usage",
			body: streamingBodyWithoutUsage,
			wantErr: false,
		},
		{
			name: "streaming request with usage",
			body: streamingBodyWithUsage,
			wantErr: false,
			want: fwkrq.Usage{
				PromptTokens:     7,
				TotalTokens:      17,
				CompletionTokens: 10,
			},
		},
		{
			name: "streaming request with usage and cached tokens",
			body: streamingBodyWithUsageAndCachedTokens,
			wantErr: false,
			want: fwkrq.Usage{
				PromptTokens:     7,
				TotalTokens:      17,
				CompletionTokens: 10,
				PromptTokenDetails: &fwkrq.PromptTokenDetails{
					CachedTokens: 5,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parser := NewParser()
			_, usage, _, err := parser.ParseStreamResponse([]byte(test.body))
			if err != nil {
				if !test.wantErr {
					t.Fatalf("ParseStreamResponse returned unexpected error: %v", err)
				}
			}

			if diff := cmp.Diff(test.want, usage); diff != "" {
				t.Errorf("ParseStreamResponse returned unexpected usage, diff(-want, +got): %v", diff)
			}
		})
	}
}

func TestParseStreamResponse_TokenAccumulation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		chunks    []string
		wantUsage fwkrq.Usage
	}{
		{
			name: "Standard: Usage and DONE in same chunk",
			chunks: []string{
				`data: {"usage":{"prompt_tokens":5,"completion_tokens":10,"total_tokens":15}}` + "\n" + `data: [DONE]`,
			},
			wantUsage: fwkrq.Usage{PromptTokens: 5, CompletionTokens: 10, TotalTokens: 15},
		},
		{
			name: "Split: Usage in Chunk 1, DONE in Chunk 2",
			chunks: []string{
				// Chunk 1: Usage data arrives
				`data: {"usage":{"prompt_tokens":5,"completion_tokens":10,"total_tokens":15}}` + "\n",
				// Chunk 2: Stream termination. Should NOT overwrite the usage from Chunk 1.
				`data: [DONE]`,
			},
			wantUsage: fwkrq.Usage{PromptTokens: 5, CompletionTokens: 10, TotalTokens: 15},
		},
		{
			name: "Fragmented: Content -> Usage -> DONE",
			chunks: []string{
				`data: {"choices":[{"text":"Hello"}]}` + "\n",
				`data: {"usage":{"prompt_tokens":5,"completion_tokens":10,"total_tokens":15}}` + "\n",
				`data: [DONE]`,
			},
			wantUsage: fwkrq.Usage{PromptTokens: 5, CompletionTokens: 10, TotalTokens: 15},
		},
		{
			name: "No Usage Data",
			chunks: []string{
				`data: {"choices":[{"text":"Hello"}]}` + "\n",
				`data: [DONE]`,
			},
			wantUsage: fwkrq.Usage{}, // Zero values
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parser := NewParser()
			var usage fwkrq.Usage
			var isComplete bool

			for _, chunk := range tc.chunks {
				_, chunkUsage, complete, _ := parser.ParseStreamResponse([]byte(chunk))
				if chunkUsage.TotalTokens > 0 {
					usage = chunkUsage
				}
				if complete {
					isComplete = true
				}
			}

			assert.Equal(t, tc.wantUsage, usage, "Usage data should match expected accumulation")
			assert.True(t, isComplete, "Response should be marked complete after [DONE]")
		})
	}
}
