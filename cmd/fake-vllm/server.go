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

package main

import (
	"context"
	"log"
	"time"

	vllm "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/grpc/gen"
)

// FakeVllmEngineServer is a fake implementation of the VllmEngine gRPC service.
type FakeVllmEngineServer struct {
	vllm.UnimplementedVllmEngineServer
}

// Generate implements the Generate method for both streaming and non-streaming mode.
// If req.Stream is true, it sends several chunks before a final complete response.
// If req.Stream is false, it sends only a final complete response.
func (s *FakeVllmEngineServer) Generate(req *vllm.GenerateRequest, stream vllm.VllmEngine_GenerateServer) error {
	log.Println("Generate is invoked", "reqStream is", req.Stream, "text is", req.GetText())
	if req.Stream {
		// Streaming mode: send multiple chunks
		for i := 0; i < 5; i++ {
			chunk := &vllm.GenerateResponse{
				Response: &vllm.GenerateResponse_Chunk{
					Chunk: &vllm.GenerateStreamChunk{
						TokenIds:         []uint32{uint32(100 + i)},
						PromptTokens:     10,
						CompletionTokens: uint32(i + 1),
					},
				},
			}
			if err := stream.Send(chunk); err != nil {
				return err
			}
			// Small delay to simulate streaming
			time.Sleep(50 * time.Millisecond)
		}
	}
	log.Println("Done generating chunk.")
	// Send final completion message (also used for non-streaming mode)
	complete := &vllm.GenerateResponse{
		Response: &vllm.GenerateResponse_Complete{
			Complete: &vllm.GenerateComplete{
				OutputIds:        []uint32{100, 101, 102, 103, 104},
				FinishReason:     "stop",
				PromptTokens:     10,
				CompletionTokens: 5,
			},
		},
	}
	return stream.Send(complete)
}

// HealthCheck implements the HealthCheck method.
func (s *FakeVllmEngineServer) HealthCheck(ctx context.Context, req *vllm.HealthCheckRequest) (*vllm.HealthCheckResponse, error) {
	return &vllm.HealthCheckResponse{
		Healthy: true,
		Message: "Fake vLLM server is healthy",
	}, nil
}
