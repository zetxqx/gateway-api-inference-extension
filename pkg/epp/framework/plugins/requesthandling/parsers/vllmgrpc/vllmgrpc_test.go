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

package vllmgrpc

import (
	"context"
	"encoding/binary"
	"testing"

	"google.golang.org/protobuf/proto"

	// Note: Adjust these imports if your local aliases differ
	pb "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/api/gen"
)

// helper function to simulate the gRPC payload framing
// [1 byte compression flag] [4 bytes message length] [message bytes...]
func createGrpcPayload(t *testing.T, msg proto.Message) []byte {
	t.Helper()
	b, err := proto.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal proto: %v", err)
	}

	payload := make([]byte, 5+len(b))
	payload[0] = 0 // 0 = uncompressed
	binary.BigEndian.PutUint32(payload[1:5], uint32(len(b)))
	copy(payload[5:], b)

	return payload
}

func TestVllmGRPCParser_PluginLifecycle(t *testing.T) {
	parser := NewVllmGRPCParser()
	if parser.TypedName().Type != VllmGRPCParserType {
		t.Errorf("expected type %s, got %s", VllmGRPCParserType, parser.TypedName().Type)
	}

	parser.WithName("custom-name")
	if parser.TypedName().Name != "custom-name" {
		t.Errorf("expected name custom-name, got %s", parser.TypedName().Name)
	}

	plugin, err := VllmGRPCParserPluginFactory("factory-name", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error from factory: %v", err)
	}

	p, ok := plugin.(*VllmGRPCParser)
	if !ok {
		t.Fatalf("expected *VllmGRPCParser, got %T", plugin)
	}
	if p.TypedName().Name != "factory-name" {
		t.Errorf("expected factory-name, got %s", p.TypedName().Name)
	}
}

func TestVllmGRPCParser_ParseRequest(t *testing.T) {
	tests := []struct {
		name          string
		reqMsg        *pb.GenerateRequest
		malformedData []byte
		expectError   bool
		expectPrompt  string
	}{
		{
			name: "Valid Text Request",
			reqMsg: &pb.GenerateRequest{
				Input: &pb.GenerateRequest_Text{
					Text: "Hello world",
				},
			},
			expectError:  false,
			expectPrompt: "Hello world",
		},
		{
			name: "Valid Tokenized Request",
			reqMsg: &pb.GenerateRequest{
				Input: &pb.GenerateRequest_Tokenized{
					Tokenized: &pb.TokenizedInput{
						OriginalText: "Tokenized hello",
					},
				},
			},
			expectError:  false,
			expectPrompt: "Tokenized hello",
		},
		{
			name:          "Malformed gRPC payload (too short)",
			malformedData: []byte{0, 0, 0},
			expectError:   true,
		},
		{
			name:          "Compressed payload (unsupported)",
			malformedData: []byte{1, 0, 0, 0, 0}, // Flag 1 = compressed
			expectError:   true,
		},
	}

	parser := NewVllmGRPCParser()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var payload []byte
			if tt.malformedData != nil {
				payload = tt.malformedData
			} else {
				payload = createGrpcPayload(t, tt.reqMsg)
			}

			reqBody, err := parser.ParseRequest(ctx, payload, nil)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected an error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if reqBody == nil || reqBody.Completions == nil {
				t.Fatalf("expected non-nil LLMRequestBody and Completions")
			}

			if reqBody.Completions.Prompt != tt.expectPrompt {
				t.Errorf("expected prompt %q, got %q", tt.expectPrompt, reqBody.Completions.Prompt)
			}
		})
	}
}

func TestVllmGRPCParser_ParseResponse(t *testing.T) {
	tests := []struct {
		name                 string
		respMsg              *pb.GenerateResponse
		expectError          bool
		expectPromptTokens   int
		expectCompleteTokens int
	}{
		{
			name: "Valid Chunk Response",
			respMsg: &pb.GenerateResponse{
				Response: &pb.GenerateResponse_Chunk{
					Chunk: &pb.GenerateStreamChunk{
						TokenIds:         []uint32{1, 2, 3},
						PromptTokens:     10,
						CompletionTokens: 5,
						CachedTokens:     2,
					},
				},
			},
			expectError:          false,
			expectPromptTokens:   10,
			expectCompleteTokens: 5,
		},
		{
			name: "Valid Complete Response",
			respMsg: &pb.GenerateResponse{
				Response: &pb.GenerateResponse_Complete{
					Complete: &pb.GenerateComplete{
						FinishReason:     "stop",
						PromptTokens:     20,
						CompletionTokens: 15,
						CachedTokens:     5,
					},
				},
			},
			expectError:          false,
			expectPromptTokens:   20,
			expectCompleteTokens: 15,
		},
		{
			name: "Empty Chunk (No Tokens, Streaming intermediate)",
			respMsg: &pb.GenerateResponse{
				Response: &pb.GenerateResponse_Chunk{
					Chunk: &pb.GenerateStreamChunk{
						TokenIds: []uint32{4},
						// PromptTokens and CompletionTokens are 0
					},
				},
			},
			expectError:        false,
			expectPromptTokens: 0, // Should result in nil Usage
		},
	}

	parser := NewVllmGRPCParser()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := createGrpcPayload(t, tt.respMsg)

			parsedResp, err := parser.ParseResponse(ctx, payload, nil, false)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected an error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectPromptTokens > 0 {
				if parsedResp.Usage == nil {
					t.Fatalf("expected Usage to be populated, got nil")
				}
				if parsedResp.Usage.PromptTokens != tt.expectPromptTokens {
					t.Errorf("expected %d prompt tokens, got %d", tt.expectPromptTokens, parsedResp.Usage.PromptTokens)
				}
				if parsedResp.Usage.CompletionTokens != tt.expectCompleteTokens {
					t.Errorf("expected %d completion tokens, got %d", tt.expectCompleteTokens, parsedResp.Usage.CompletionTokens)
				}
			} else if parsedResp.Usage != nil {
				t.Errorf("expected Usage to be nil for empty/intermediate chunks, got %+v", parsedResp.Usage)
			}
		})
	}
}
