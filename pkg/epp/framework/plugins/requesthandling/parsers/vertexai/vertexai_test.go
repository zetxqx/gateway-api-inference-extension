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

package vertexai

import (
	"context"
	"encoding/binary"
	"strings"
	"testing"

	"cloud.google.com/go/aiplatform/apiv1beta1/aiplatformpb"
	"google.golang.org/genproto/googleapis/api/httpbody"
	"google.golang.org/protobuf/proto"
	v1 "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	fwkrh "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/requesthandling"
)

func TestParseRequest_ChatCompletions(t *testing.T) {
	parser := NewVertexAIParser()
	
	jsonPayload := []byte(`{"messages":[{"role":"user","content":"Hello"}],"stream":true}`)
	
	reqMsg := &aiplatformpb.ChatCompletionsRequest{
		HttpBody: &httpbody.HttpBody{
			Data: jsonPayload,
		},
	}
	
	body, err := createGrpcFrame(reqMsg)
	if err != nil {
		t.Fatalf("Failed to create gRPC frame: %v", err)
	}

	headers := map[string]string{":path": "/google.cloud.aiplatform.v1beta1.PredictionService/ChatCompletions"}

	res, err := parser.ParseRequest(context.Background(), body, headers)
	if err != nil {
		t.Fatalf("ParseRequest failed: %v", err)
	}
	req := res.Body

	if req.ChatCompletions == nil {
		t.Fatal("Expected ChatCompletions to be populated")
	}
	if req.ChatCompletions.Messages[0].Content.Raw != "Hello" {
		t.Errorf("Expected prompt 'Hello', got %v", req.ChatCompletions.Messages[0].Content.Raw)
	}
	if !req.Stream {
		t.Error("Expected Stream to be true")
	}

	if req.Payload == nil {
		t.Fatal("Expected Payload to be populated")
	}
	payloadProto, ok := req.Payload.(fwkrh.PayloadProto)
	if !ok {
		t.Fatal("Expected Payload to be of type fwkrh.PayloadProto")
	}
	if !proto.Equal(payloadProto.Message, reqMsg) {
		t.Errorf("Expected Payload Message to be %v, got %v", reqMsg, payloadProto.Message)
	}
}

func createGrpcFrame(msg proto.Message) ([]byte, error) {
	payload, err := proto.Marshal(msg)
	if err != nil {
		return nil, err
	}
	header := make([]byte, 5)
	header[0] = 0 // uncompressed
	binary.BigEndian.PutUint32(header[1:], uint32(len(payload)))
	return append(header, payload...), nil
}

func TestParseRequest_Errors(t *testing.T) {
	parser := NewVertexAIParser()

	tests := []struct {
		name              string
		body              []byte
		headers           map[string]string
		expectedErr       string
		wantBypassOnError bool
	}{
		{
			name:              "Unsupported gRPC path",
			body:              []byte{},
			headers:           map[string]string{":path": "/unsupported/path"},
			expectedErr:       "unsupported gRPC path",
			wantBypassOnError: true,
		},
		{
			name:              "Invalid gRPC frame",
			body:              []byte{0, 0, 0, 0}, // Too short
			headers:           map[string]string{":path": "/google.cloud.aiplatform.v1beta1.PredictionService/ChatCompletions"},
			expectedErr:       "parsing gRPC frame for ChatCompletions",
			wantBypassOnError: false,
		},
		{
			name:              "Invalid proto message",
			body:              []byte{0, 0, 0, 0, 1, 0xFF}, // Valid header, invalid payload
			headers:           map[string]string{":path": "/google.cloud.aiplatform.v1beta1.PredictionService/ChatCompletions"},
			expectedErr:       "unmarshaling ChatCompletionsRequest",
			wantBypassOnError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := parser.ParseRequest(context.Background(), tc.body, tc.headers)
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			if res == nil {
				t.Fatal("ParseRequest() returned nil result on error")
			}
			if res.BypassOnError != tc.wantBypassOnError {
				t.Errorf("ParseRequest() BypassOnError = %v, want %v", res.BypassOnError, tc.wantBypassOnError)
			}
			if !strings.Contains(err.Error(), tc.expectedErr) {
				t.Errorf("Expected error containing %q, got %v", tc.expectedErr, err)
			}
		})
	}
}

func TestParseResponse(t *testing.T) {
	parser := NewVertexAIParser()

	jsonPayload := []byte(`{"object":"chat.completion","usage":{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}}`)
	httpBody := &httpbody.HttpBody{
		Data: jsonPayload,
	}
	httpBodyBytes, err := proto.Marshal(httpBody)
	if err != nil {
		t.Fatalf("Failed to marshal HttpBody: %v", err)
	}
	validBody, err := createGrpcFrameRaw(httpBodyBytes)
	if err != nil {
		t.Fatalf("Failed to create gRPC frame: %v", err)
	}

	invalidProtoBody, _ := createGrpcFrameRaw([]byte{0xFF})

	tests := []struct {
		name          string
		body          []byte
		headers       map[string]string
		expectedErr   string
		expectedUsage *fwkrh.Usage
	}{
		{
			name:          "Empty body",
			body:          []byte{},
			headers:       nil,
			expectedErr:   "",
			expectedUsage: nil,
		},
		{
			name:          "Valid JSON response",
			body:          validBody,
			headers:       nil,
			expectedErr:   "",
			expectedUsage: &fwkrh.Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
		},
		{
			name:          "Invalid gRPC frame",
			body:          []byte{0, 0, 0, 0},
			headers:       nil,
			expectedErr:   "parsing gRPC frame for response",
			expectedUsage: nil,
		},
		{
			name:          "Invalid proto message",
			body:          invalidProtoBody,
			headers:       nil,
			expectedErr:   "unmarshaling HttpBody response",
			expectedUsage: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := parser.ParseResponse(context.Background(), tc.body, tc.headers, false)
			if tc.expectedErr != "" {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.expectedErr) {
					t.Errorf("Expected error containing %q, got %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseResponse failed: %v", err)
			}
			if tc.expectedUsage == nil {
				if resp != nil {
					t.Errorf("Expected nil response, got %v", resp)
				}
			} else {
				if resp == nil {
					t.Fatal("Expected non-nil response")
				}
				if resp.Usage == nil {
					t.Fatal("Expected Usage to be populated")
				}
				if resp.Usage.PromptTokens != tc.expectedUsage.PromptTokens {
					t.Errorf("Expected prompt tokens %d, got %d", tc.expectedUsage.PromptTokens, resp.Usage.PromptTokens)
				}
			}
		})
	}
}

func TestVertexAIParser_Metadata(t *testing.T) {
	parser := NewVertexAIParser()

	typedName := parser.TypedName()
	if typedName.Type != VertexAIParserType {
		t.Errorf("Expected type %s, got %s", VertexAIParserType, typedName.Type)
	}
	if typedName.Name != VertexAIParserType {
		t.Errorf("Expected name %s, got %s", VertexAIParserType, typedName.Name)
	}

	protocols := parser.SupportedAppProtocols()
	if len(protocols) != 1 || protocols[0] != v1.AppProtocolH2C {
		t.Errorf("Expected protocols [h2c], got %v", protocols)
	}
}

func TestVertexAIParserPluginFactory(t *testing.T) {
	plugin, err := VertexAIParserPluginFactory("test-parser", nil, nil)
	if err != nil {
		t.Fatalf("VertexAIParserPluginFactory failed: %v", err)
	}
	parser, ok := plugin.(*VertexAIParser)
	if !ok {
		t.Fatal("Expected plugin to be of type *VertexAIParser")
	}
	if parser.TypedName().Name != "test-parser" {
		t.Errorf("Expected name 'test-parser', got %s", parser.TypedName().Name)
	}
}

func createGrpcFrameRaw(payload []byte) ([]byte, error) {
	header := make([]byte, 5)
	header[0] = 0 // uncompressed
	binary.BigEndian.PutUint32(header[1:], uint32(len(payload)))
	return append(header, payload...), nil
}
