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
	"io"
	"net"
	"strconv"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	vllm "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/grpc/gen"
)

func TestFakeVllmEngineServer(t *testing.T) {
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	port := lis.Addr().(*net.TCPAddr).Port

	s := grpc.NewServer()
	vllm.RegisterVllmEngineServer(s, &FakeVllmEngineServer{})
	go func() {
		if err := s.Serve(lis); err != nil {
			t.Errorf("Server failed: %v", err)
		}
	}()
	defer s.Stop()

	conn, err := grpc.NewClient(net.JoinHostPort("localhost", strconv.Itoa(port)), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	client := vllm.NewVllmEngineClient(conn)

	t.Run("HealthCheck", func(t *testing.T) {
		resp, err := client.HealthCheck(context.Background(), &vllm.HealthCheckRequest{})
		if err != nil {
			t.Fatalf("HealthCheck failed: %v", err)
		}
		if !resp.Healthy {
			t.Errorf("expected healthy, got unhealthy")
		}
	})

	t.Run("GenerateNonStreaming", func(t *testing.T) {
		stream, err := client.Generate(context.Background(), &vllm.GenerateRequest{
			RequestId: "test-non-stream",
			Stream:    false,
		})
		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}

		resp, err := stream.Recv()
		if err != nil {
			t.Fatalf("Recv failed: %v", err)
		}

		if _, ok := resp.Response.(*vllm.GenerateResponse_Complete); !ok {
			t.Errorf("expected complete response, got %T", resp.Response)
		}

		_, err = stream.Recv()
		if err != io.EOF {
			t.Errorf("expected EOF, got %v", err)
		}
	})

	t.Run("GenerateStreaming", func(t *testing.T) {
		stream, err := client.Generate(context.Background(), &vllm.GenerateRequest{
			RequestId: "test-stream",
			Stream:    true,
		})
		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}

		chunkCount := 0
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("Recv failed: %v", err)
			}

			switch resp.Response.(type) {
			case *vllm.GenerateResponse_Chunk:
				chunkCount++
			case *vllm.GenerateResponse_Complete:
				if chunkCount != 5 {
					t.Errorf("expected 5 chunks before complete, got %d", chunkCount)
				}
			}
		}
	})
}
