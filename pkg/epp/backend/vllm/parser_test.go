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

package vllm

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"

	vllmpb "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/api/gen"
	fwkrq "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/requestcontrol"
)

func TestParseRequest(t *testing.T) {
	parser := NewParser()

	// Case 1: Text input, no model in headers
	req1 := &vllmpb.GenerateRequest{
		Input: &vllmpb.GenerateRequest_Text{Text: "Hello"},
	}
	body1, _ := proto.Marshal(req1)
	headers1 := map[string]string{}
	map1, err := parser.ParseRequest(body1, headers1)
	if err != nil {
		t.Fatalf("ParseRequest failed: %v", err)
	}
	if map1["prompt"] != "Hello" {
		t.Errorf("expected prompt 'Hello', got %v", map1["prompt"])
	}
	if map1["model"] != nil {
		t.Errorf("expected model to be nil, got %v", map1["model"])
	}

	// Case 2: Tokenized input, model in headers
	req2 := &vllmpb.GenerateRequest{
		Input: &vllmpb.GenerateRequest_Tokenized{
			Tokenized: &vllmpb.TokenizedInput{OriginalText: "Original"},
		},
	}
	body2, _ := proto.Marshal(req2)
	headers2 := map[string]string{"x-model-name": "my-model"}
	map2, err := parser.ParseRequest(body2, headers2)
	if err != nil {
		t.Fatalf("ParseRequest failed: %v", err)
	}
	if map2["prompt"] != "Original" {
		t.Errorf("expected prompt 'Original', got %v", map2["prompt"])
	}
	if map2["model"] != "my-model" {
		t.Errorf("expected model 'my-model', got %v", map2["model"])
	}
}

func TestParseResponse(t *testing.T) {
	parser := NewParser()

	resp := &vllmpb.GenerateResponse{
		Response: &vllmpb.GenerateResponse_Complete{
			Complete: &vllmpb.GenerateComplete{
				PromptTokens:     10,
				CompletionTokens: 20,
				CachedTokens:     5,
			},
		},
	}
	body, _ := proto.Marshal(resp)
	respMap, usage, err := parser.ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}

	want := fwkrq.Usage{
		PromptTokens:     10,
		CompletionTokens: 20,
		TotalTokens:      30,
		PromptTokenDetails: &fwkrq.PromptTokenDetails{
			CachedTokens: 5,
		},
	}
	if diff := cmp.Diff(want, usage); diff != "" {
		t.Errorf("ParseResponse mismatch (-want +got):\n%s", diff)
	}

	// Verify that the map contains the response body
	completeMap, ok := respMap["complete"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'complete' key in response map, got %v", respMap)
	}
	// json unmarshal converts numbers to float64
	if pt, ok := completeMap["promptTokens"].(float64); !ok || pt != 10 {
		t.Errorf("expected promptTokens 10, got %v", completeMap["promptTokens"])
	}
}

func TestParseStreamResponse(t *testing.T) {
	parser := NewParser()

	// Chunk
	chunkResp := &vllmpb.GenerateResponse{
		Response: &vllmpb.GenerateResponse_Chunk{
			Chunk: &vllmpb.GenerateStreamChunk{
				PromptTokens:     5,
				CompletionTokens: 5,
			},
		},
	}
	chunkBody, _ := proto.Marshal(chunkResp)
	usage, complete, err := parser.ParseStreamResponse(chunkBody)
	if err != nil {
		t.Fatalf("ParseStreamResponse chunk failed: %v", err)
	}
	if complete {
		t.Error("expected chunk to be incomplete")
	}
	if usage.PromptTokens != 5 || usage.CompletionTokens != 5 {
		t.Errorf("usage mismatch for chunk: %v", usage)
	}

	// Complete
	completeResp := &vllmpb.GenerateResponse{
		Response: &vllmpb.GenerateResponse_Complete{
			Complete: &vllmpb.GenerateComplete{
				PromptTokens:     10,
				CompletionTokens: 20,
			},
		},
	}
	completeBody, _ := proto.Marshal(completeResp)
	usage, complete, err = parser.ParseStreamResponse(completeBody)
	if err != nil {
		t.Fatalf("ParseStreamResponse complete failed: %v", err)
	}
	if !complete {
		t.Error("expected complete to be true")
	}
	if usage.PromptTokens != 10 || usage.CompletionTokens != 20 {
		t.Errorf("usage mismatch for complete: %v", usage)
	}
}
