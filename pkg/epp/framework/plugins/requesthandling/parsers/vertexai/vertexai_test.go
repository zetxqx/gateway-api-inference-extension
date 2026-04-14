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
	"testing"

	"cloud.google.com/go/aiplatform/apiv1beta1/aiplatformpb"
	"google.golang.org/genproto/googleapis/api/httpbody"
	"google.golang.org/protobuf/proto"
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

	req, err := parser.ParseRequest(context.Background(), body, headers)
	if err != nil {
		t.Fatalf("ParseRequest failed: %v", err)
	}

	if req.ChatCompletions == nil {
		t.Fatal("Expected ChatCompletions to be populated")
	}
	if req.ChatCompletions.Messages[0].Content.Raw != "Hello" {
		t.Errorf("Expected prompt 'Hello', got %v", req.ChatCompletions.Messages[0].Content.Raw)
	}
	if !req.Stream {
		t.Error("Expected Stream to be true")
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
