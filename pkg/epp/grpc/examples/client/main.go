package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	vllm "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/grpc/gen"
)

const (
	// address = "35.212.230.187:443" // l7lb
	// address = "34.82.30.251:443" // l4lb
	// address = "35.212.202.130:443" // igw
	// address = "35.212.234.191:80" // igw no tls
	address = "34.169.180.33:8081" // standalone
)

func secureConn() *grpc.ClientConn {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}
	creds := credentials.NewTLS(tlsConfig)

	// Set up a connection to the server using the TLS creds
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	return conn
}

func insecureConn() *grpc.ClientConn {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	return conn
}

func main() {
	conn := insecureConn()
	defer conn.Close()
	c := vllm.NewVllmEngineClient(conn)

	// Use a longer timeout for generation
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Call GetModelInfo
	getModelInfo(ctx, c)

	// Initialize Tokenizer
	tokenizer := NewTokenizer()

	// 2. Call Generate (Non-Streaming)
	generateNonStreaming(ctx, c, tokenizer)

	// 3. Call Generate (Streaming)
	generateStreaming(ctx, c, tokenizer)
}

func getModelInfo(ctx context.Context, c vllm.VllmEngineClient) {
	fmt.Println("--- GetModelInfo ---")
	r, err := c.GetModelInfo(ctx, &vllm.GetModelInfoRequest{})
	if err != nil {
		log.Printf("could not get model info: %v", err)
		return
	}
	fmt.Printf("Model Info: Path=%s, MaxContext=%d\n", r.GetModelPath(), r.GetMaxContextLength())
}

func generateNonStreaming(ctx context.Context, c vllm.VllmEngineClient, t *Tokenizer) {
	fmt.Println("\n--- Generate (Non-Streaming) ---")
	maxTokens := int32(50)
	req := &vllm.GenerateRequest{
		RequestId: "req-non-stream",
		Tokenized: &vllm.TokenizedInput{
			OriginalText: "Hello, world!",
			InputIds:     []uint32{1, 2, 3, 4, 5}, // Dummy tokens
		},
		SamplingParams: &vllm.SamplingParams{
			Temperature: 0.7,
			MaxTokens:   &maxTokens,
		},
		Stream: false,
	}

	stream, err := c.Generate(ctx, req)
	if err != nil {
		log.Printf("error calling Generate: %v", err)
		return
	}

	// For non-streaming, we expect one response with 'Complete' or 'Error'
	resp, err := stream.Recv()
	if err == io.EOF {
		log.Println("stream closed without response")
		return
	}
	if err != nil {
		log.Printf("error receiving response: %v", err)
		return
	}

	// Handle the oneof Response
	switch r := resp.Response.(type) {
	case *vllm.GenerateResponse_Complete:
		fmt.Printf("Complete Response (IDs): %v\n", r.Complete.OutputIds)
		fmt.Printf("Complete Response (Text): %s\n", t.Detokenize(r.Complete.OutputIds))
		fmt.Printf("Finish Reason: %s\n", r.Complete.FinishReason)
	case *vllm.GenerateResponse_Error:
		fmt.Printf("Error Response: %s\n", r.Error.Message)
	case *vllm.GenerateResponse_Chunk:
		fmt.Printf("Unexpected Chunk Response in non-streaming mode: %v\n", r.Chunk)
	default:
		fmt.Printf("Unknown response type\n")
	}

	// Verify stream is closed
	_, err = stream.Recv()
	if err != io.EOF {
		log.Printf("expected EOF, got: %v", err)
	}
}

func generateStreaming(ctx context.Context, c vllm.VllmEngineClient, t *Tokenizer) {
	fmt.Println("\n--- Generate (Streaming) ---")
	maxTokens := int32(50)
	req := &vllm.GenerateRequest{
		RequestId: "req-stream",
		Tokenized: &vllm.TokenizedInput{
			OriginalText: "Tell me a story.",
			InputIds:     []uint32{10, 11, 12, 13}, // Dummy tokens
		},
		SamplingParams: &vllm.SamplingParams{
			Temperature: 0.7,
			MaxTokens:   &maxTokens,
		},
		Stream: true,
	}

	stream, err := c.Generate(ctx, req)
	if err != nil {
		log.Printf("error calling Generate: %v", err)
		return
	}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("error receiving stream: %v", err)
			return
		}

		switch r := resp.Response.(type) {
		case *vllm.GenerateResponse_Chunk:
			text := t.Detokenize(r.Chunk.TokenIds)
			fmt.Printf("Received chunk: %v -> \"%s\"\n", r.Chunk.TokenIds, text)
		case *vllm.GenerateResponse_Error:
			fmt.Printf("Received error: %s\n", r.Error.Message)
			return
		case *vllm.GenerateResponse_Complete:
			// Depending on vLLM version/implementation, might send a final complete message or just end.
			fmt.Printf("Stream completed with final info: %v\n", r.Complete)
		}
	}
	fmt.Println("Streaming finished.")
}
