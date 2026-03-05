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
	"encoding/json"
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"
	"sigs.k8s.io/controller-runtime/pkg/log"
	logutil "sigs.k8s.io/gateway-api-inference-extension/pkg/common/observability/logging"
	pb "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/api/gen"
	fwkplugin "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/plugin"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/requestcontrol"
	fwkrh "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/requesthandling"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/scheduling"
)

const (
	VllmGRPCParserType = "vllmgrpc-parser"
)

// compile-time type validation
var _ fwkrh.Parser = &VllmGRPCParser{}

// VllmGRPCParser implements the fwkrh.Parser interface for vLLM gRPC.
type VllmGRPCParser struct {
	typedName fwkplugin.TypedName
}

// NewVllmGRPCParser creates a new VllmGRPCParser.
func NewVllmGRPCParser() *VllmGRPCParser {
	return &VllmGRPCParser{
		typedName: fwkplugin.TypedName{
			Type: VllmGRPCParserType,
			Name: VllmGRPCParserType,
		},
	}
}

func VllmGRPCParserPluginFactory(name string, _ json.RawMessage, _ fwkplugin.Handle) (fwkplugin.Plugin, error) {
	return NewVllmGRPCParser().WithName(name), nil
}

func (p *VllmGRPCParser) WithName(name string) *VllmGRPCParser {
	p.typedName.Name = name
	return p
}

// TypedName returns the type and name tuple of this plugin instance.
func (p *VllmGRPCParser) TypedName() fwkplugin.TypedName {
	return p.typedName
}

// ParseRequest parses the gRPC request body and headers and returns an LLMRequestBody.
func (p *VllmGRPCParser) ParseRequest(ctx context.Context, body []byte, headers map[string]string) (*scheduling.LLMRequestBody, error) {
	logger := log.FromContext(ctx)

	extractedBody, err := convertToLLMRequestBody(body)
	logger.V(logutil.DEBUG).Info("parsed GenerateRequest")
	if err != nil {
		return nil, fmt.Errorf("failed to parse gRPC payload: %w", err)
	}
	return extractedBody, nil
}

// ParseResponse parses the response body and returns a ParsedResponse
func (p *VllmGRPCParser) ParseResponse(ctx context.Context, body []byte, headers map[string]string, endofStream bool) (*fwkrh.ParsedResponse, error) {
	logger := log.FromContext(ctx)
	pbResp, err := toGenerateResponse(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse gRPC response payload: %w", err)
	}

	result := &fwkrh.ParsedResponse{}

	if pbResp != nil && pbResp.Response != nil {
		switch v := pbResp.Response.(type) {
		case *pb.GenerateResponse_Chunk:
			logger.V(logutil.DEBUG).Info("parsed GenerateResponse_Chunk. Token IDs length: ", "tokenLength", len(v.Chunk.TokenIds))
			// Only populate Usage if the chunk actually contains token data.
			// Streaming chunks often leave this empty until the final chunk.
			if v.Chunk.PromptTokens > 0 || v.Chunk.CompletionTokens > 0 {
				result.Usage = &requestcontrol.Usage{
					PromptTokens:     int(v.Chunk.PromptTokens),
					CompletionTokens: int(v.Chunk.CompletionTokens),
					TotalTokens:      int(v.Chunk.PromptTokens + v.Chunk.CompletionTokens),
					PromptTokenDetails: &requestcontrol.PromptTokenDetails{
						CachedTokens: int(v.Chunk.CachedTokens),
					},
				}
			}

		case *pb.GenerateResponse_Complete:
			logger.V(logutil.DEBUG).Info("parsed GenerateResponse_Complete. Token IDs length: ", "finishedReason", len(v.Complete.FinishReason))
			// Populate Usage for complete, non-streaming responses.
			if v.Complete.PromptTokens > 0 || v.Complete.CompletionTokens > 0 {
				result.Usage = &requestcontrol.Usage{
					PromptTokens:     int(v.Complete.PromptTokens),
					CompletionTokens: int(v.Complete.CompletionTokens),
					TotalTokens:      int(v.Complete.PromptTokens + v.Complete.CompletionTokens),
					PromptTokenDetails: &requestcontrol.PromptTokenDetails{
						CachedTokens: int(v.Complete.CachedTokens),
					},
				}
			}

		default:
			return nil, errors.New("unrecognized response type in GenerateResponse")
		}
	}

	return result, nil
}

// toGenerateResponse extracts the gRPC payload and unmarshals it into a GenerateResponse.
func toGenerateResponse(payload []byte) (*pb.GenerateResponse, error) {
	parsedPayload, compressed, ok := parseGrpcPayload(payload)
	if !ok {
		return nil, errors.New("not able to parse payload")
	}
	if compressed {
		// TODO: handle compressed payload.
		return nil, errors.New("not able to parse compressed payload")
	}

	resp := &pb.GenerateResponse{}
	err := proto.Unmarshal(parsedPayload, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func convertToLLMRequestBody(payload []byte) (*scheduling.LLMRequestBody, error) {
	pbReq, err := toGenerateRequest(payload)
	if err != nil {
		return nil, err
	}
	if pbReq == nil {
		return nil, err
	}
	switch pbReq.Input.(type) {
	case *pb.GenerateRequest_Text:
		return &scheduling.LLMRequestBody{
			Completions: &scheduling.CompletionsRequest{
				Prompt: pbReq.GetText(),
			},
			ParsedBody: pbReq,
		}, nil
	case *pb.GenerateRequest_Tokenized:
		return &scheduling.LLMRequestBody{
			Completions: &scheduling.CompletionsRequest{
				Prompt: pbReq.GetTokenized().OriginalText,
			},
			ParsedBody: pbReq,
		}, nil
	}
	return nil, errors.New("not supported request inputType")
}

// parseGrpcPayload extracts the payload and the compression status.
//
// Returns: (payload, isCompressed, success)
func parseGrpcPayload(data []byte) ([]byte, bool, bool) {
	if len(data) < 5 {
		return nil, false, false
	}

	// gRPC frame header: [Compression Flag (1 byte)] [Message Length (4 bytes)]
	// Compression Flag 0 = uncompressed, 1 = compressed
	isCompressed := data[0] == 1
	msgLen := binary.BigEndian.Uint32(data[1:5])

	if uint32(len(data)) < 5+msgLen {
		return nil, false, false
	}
	return data[5 : 5+msgLen], isCompressed, true
}

func toGenerateRequest(payload []byte) (*pb.GenerateRequest, error) {
	parsedPayload, compressed, ok := parseGrpcPayload(payload)
	if !ok {
		return nil, errors.New("not able to parse payload")
	}
	if compressed {
		// TODO: handle compressed payload.
		return nil, errors.New("not able to parse compressed payload")
	}
	req := &pb.GenerateRequest{}
	err := proto.Unmarshal(parsedPayload, req)
	if err != nil {
		return nil, err
	}
	return req, nil
}
