package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"

	configPb "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	v3pb "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extProcPb "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	pb "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/grpc/gen"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/metadata"
)

// Helper: Parse gRPC 5-byte framing
func parseGrpcPayload(data []byte) ([]byte, bool) {
	if len(data) >= 5 { // ">=" is very important, because grpc may contain nothing in the body.
		msgLen := binary.BigEndian.Uint32(data[1:5])
		if uint32(len(data)) >= 5+msgLen {
			return data[5 : 5+msgLen], true
		}
	}
	return nil, false
}

// Helper: Encode gRPC payload frame
func encodeGrpcPayload(data []byte) error {
	// jsonReq := &ChatCompletionRequest{}
	// if err := json.Unmarshal(requestBodyBuffer, jsonReq); err != nil {
	// 	log.Println("[JSON Parser] Error unmrshalling the JSON request", err)
	// 	return err
	// }
	// protoReq := convertToGenerateRequest(*jsonReq)
	// protoBytes, err := proto.Marshal(protoReq)
	// if err != nil {
	// 	log.Println("[Proto Parser] Error marshal the GenerateRequest proto message", err)
	// 	return err
	// }
	// // gRPC 5-byte Prefix (Compression Flag + 4-byte Length)
	// grpcFrame := make([]byte, 5)
	// grpcFrame[0] = 0 // No compression
	// binary.BigEndian.PutUint32(grpcFrame[1:], uint32(len(protoBytes)))
	// // Change the requestBody to proto format
	// requestBodyBuffer = append(grpcFrame, protoBytes...)
	return nil
}

type extProcServer struct {
	targetIP          string
	enableHttpConvert bool
}

func (s *extProcServer) Process(stream extProcPb.ExternalProcessor_ProcessServer) error {
	ctx := stream.Context()

	// State variables per stream
	var grpcMethod string
	var contentType string
	var requestBodyBuffer []byte
	var responseBodyBuffer []byte
	var contentTypeToSet string
	var httpMethod string

	log.Println("[Stream Start] New processing stream started")

	for {
		select {
		case <-ctx.Done():
			log.Printf("[Stream End] Context cancelled: %v\n", ctx.Err())
			return ctx.Err()
		default:
		}

		req, err := stream.Recv()
		if err == io.EOF {
			log.Println("[Stream End] Client closed stream (EOF)")
			return nil
		}
		if err != nil {
			log.Printf("[Stream Error] Receive error: %v\n", err)
			return status.Errorf(codes.Unknown, "cannot receive stream request: %v", err)
		}

		responseBodyResps := []*extProcPb.ProcessingResponse{} // For response body only.
		resp := &extProcPb.ProcessingResponse{}
		phase := "Unknown"
		log.Println("[Info] Back to switch case")

		switch v := req.Request.(type) {

		// ----------------------------------------------------------------
		// HEADER PHASE
		// ----------------------------------------------------------------
		case *extProcPb.ProcessingRequest_RequestHeaders:
			phase = "RequestHeaders"
			log.Println(">>> [Phase: RequestHeaders] <<<")
			log.Printf(" Printing all %d headers:", len(v.RequestHeaders.Headers.Headers))
			// gRPC header:
			//   1. chat/completions, non-streaming: https://screenshot.googleplex.com/7LkqWXsXXgsBuGk
			//.  2. streaming: https://screenshot.googleplex.com/4CCPj4RyETX82Eu
			//.  3. GetModelInfo: https://screenshot.googleplex.com/6ejoBeBUQtrXc4t
			// HTTP header:
			//   1. chat/completions, non-streaming: https://screenshot.googleplex.com/5sXYJLGtuPgu6f8
			//   2. /models : https://screenshot.googleplex.com/AhWiFc86SMRhQdr (no request body at all for GET http)
			//.  3. streaming chat/completions: https://screenshot.googleplex.com/8QDoSondfYPX75A
			for _, h := range v.RequestHeaders.Headers.Headers {
				log.Printf("    - Key=%q Value=%q RawValue=%q", h.Key, h.Value, h.RawValue)
				if h.Key == ":path" {
					grpcMethod = string(h.RawValue)
				}
				if h.Key == "content-type" {
					contentType = string(h.RawValue)
				}
				if h.Key == ":method" {
					// This should eb only "GET" or "POST"
					httpMethod = string(h.RawValue)
				}
			}

			log.Printf("    Method Found: %s\n", grpcMethod)
			if httpMethod == "GET" || grpcMethod == "/v1/models" { // Have to use OR for a workround when client send this request in POST manner.
				// Special handle for GET request for "v1/models"
				log.Println("Special handle for GET request or /v1/models request")
				if s.enableHttpConvert {
					log.Println("[JSON Parser] Parsing JSON request /v1/models")
					protoReq := &pb.GetModelInfoRequest{}
					protoBytes, err := proto.Marshal(protoReq)
					if err != nil {
						log.Println("[Proto Parser] Error marshal the GetModelInfo proto message", err)
						return err
					}
					// gRPC 5-byte Prefix (Compression Flag + 4-byte Length)
					grpcFrame := make([]byte, 5)
					grpcFrame[0] = 0 // No compression
					binary.BigEndian.PutUint32(grpcFrame[1:], uint32(len(protoBytes)))
					// Change the requestBody to proto format
					requestBodyBuffer = append(grpcFrame, protoBytes...)
					contentTypeToSet = "application/grpc" // Set the contentType to gRPC.

					resp = &extProcPb.ProcessingResponse{
						Response: &extProcPb.ProcessingResponse_RequestHeaders{
							RequestHeaders: &extProcPb.HeadersResponse{
								Response: &extProcPb.CommonResponse{
									Status: extProcPb.CommonResponse_CONTINUE_AND_REPLACE, // VERY IMPORTANT since we use BODY MUTATION here.
									HeaderMutation: &extProcPb.HeaderMutation{
										// Try this: this seems does not work.
										RemoveHeaders: []string{"content-length"},
										SetHeaders: []*configPb.HeaderValueOption{
											{
												Header: &configPb.HeaderValue{
													Key:      metadata.DestinationEndpointKey,
													RawValue: []byte(s.targetIP),
												},
												AppendAction: configPb.HeaderValueOption_APPEND_IF_EXISTS_OR_ADD,
											},
											// Change Method to POST
											{
												// This seems not working becasue envoy not support modify ":method"? ref: https://screenshot.googleplex.com/5c3CGZjsxjcja8r
												// However even this is not mutated, as long as we use CONTINUE_AND_REPLACE the model server is still responding.
												Header: &configPb.HeaderValue{
													Key:      ":method",
													Value:    "POST", // last try
													RawValue: []byte("POST"),
												},
												AppendAction: configPb.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
											},
											// Change Path to the gRPC service method
											{
												Header:       &configPb.HeaderValue{Key: ":path", RawValue: []byte("/vllm.grpc.engine.VllmEngine/GetModelInfo")},
												AppendAction: configPb.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
											},
											// Set Content-Type
											{
												Header:       &configPb.HeaderValue{Key: "content-type", RawValue: []byte("application/grpc")},
												AppendAction: configPb.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
											},
											// // Set Content-Length
											// {
											// 	Header:       &configPb.HeaderValue{Key: "content-length", RawValue: []byte(fmt.Sprintf("%d", len(requestBodyBuffer)))},
											// 	AppendAction: configPb.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
											// },
											// Add grpc-timeout
											{
												Header: &configPb.HeaderValue{
													Key:      "grpc-timeout",
													RawValue: []byte("20S"),
												},
												AppendAction: configPb.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
											},
											// Set TE trailers
											{
												Header:       &configPb.HeaderValue{Key: "te", RawValue: []byte("trailers")},
												AppendAction: configPb.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
											},
										},
									},
									ClearRouteCache: true, // Good practice when changing Path/Method
									// Important: also add the body mutation here, since the http GET will not send a following request body as POST.
									BodyMutation: &extProcPb.BodyMutation{
										Mutation: &extProcPb.BodyMutation_Body{
											Body: requestBodyBuffer,
										},
									},
								},
							},
						},
						// Very important on processing the GET request. We need to skip bodyMode because nothing is in the following method.
						ModeOverride: &v3pb.ProcessingMode{
							RequestBodyMode:     v3pb.ProcessingMode_NONE,
							RequestTrailerMode:  v3pb.ProcessingMode_SKIP,
							ResponseHeaderMode:  v3pb.ProcessingMode_SEND,
							ResponseBodyMode:    v3pb.ProcessingMode_FULL_DUPLEX_STREAMED,
							ResponseTrailerMode: v3pb.ProcessingMode_SEND,
						},
					}
				} else {
					resp = &extProcPb.ProcessingResponse{
						Response: &extProcPb.ProcessingResponse_RequestHeaders{
							RequestHeaders: &extProcPb.HeadersResponse{
								Response: &extProcPb.CommonResponse{
									Status: extProcPb.CommonResponse_CONTINUE,
									HeaderMutation: &extProcPb.HeaderMutation{
										SetHeaders: []*configPb.HeaderValueOption{
											{
												Header: &configPb.HeaderValue{
													Key:      metadata.DestinationEndpointKey,
													RawValue: []byte(s.targetIP),
												},
												AppendAction: configPb.HeaderValueOption_APPEND_IF_EXISTS_OR_ADD,
											},
										},
									},
								},
							},
						},
						// Very important on processing the GET request.
						ModeOverride: &v3pb.ProcessingMode{
							RequestBodyMode:     v3pb.ProcessingMode_NONE,
							RequestTrailerMode:  v3pb.ProcessingMode_SKIP,
							ResponseHeaderMode:  v3pb.ProcessingMode_SEND,
							ResponseBodyMode:    v3pb.ProcessingMode_FULL_DUPLEX_STREAMED,
							ResponseTrailerMode: v3pb.ProcessingMode_SEND,
						},
					}
				}
			} else {
				continue
			}

		// ----------------------------------------------------------------
		// REQUEST BODY PHASE
		// ----------------------------------------------------------------
		case *extProcPb.ProcessingRequest_RequestBody:
			phase = "RequestBody"
			chunkSize := len(v.RequestBody.Body)
			requestBodyBuffer = append(requestBodyBuffer, v.RequestBody.Body...)
			totalSize := len(requestBodyBuffer)

			log.Printf(">>> [Phase: RequestBody] Chunk: %d bytes | Total: %d bytes | EndOfStream: %v\n",
				chunkSize, totalSize, v.RequestBody.EndOfStream)

			if !v.RequestBody.EndOfStream {
				// Continue buffering
				continue
			} else {
				// End of Stream: Process Logic
				log.Println("[Processing Logic] Full body received. Decoding...")

				var headersToSet []*configPb.HeaderValueOption
				targetPodIP := ""
				if s.targetIP != "" {
					targetPodIP = s.targetIP
					log.Printf("[Decision] Using Configured Flag Target IP: %s\n", targetPodIP)
				}

				if contentType == "application/grpc" {
					switch grpcMethod {
					case "/vllm.grpc.engine.VllmEngine/GetModelInfo":
						log.Println("Parsing /vllm.grpc.engine.VllmEngine/GetModelInfo")
						if payload, ok := parseGrpcPayload(requestBodyBuffer); ok {
							req := &pb.GetModelInfoRequest{}
							if err := proto.Unmarshal(payload, req); err == nil {
								log.Println("    [Parser] Success: GetModelInfoRequest")
							}
						} else {
							log.Println("Not valid /vllm.grpc.engine.VllmEngine/GetModelInfo")
						}

					case "/vllm.grpc.engine.VllmEngine/Generate":
						log.Println("Parsing /vllm.grpc.engine.VllmEngine/Generate")
						if payload, ok := parseGrpcPayload(requestBodyBuffer); ok {
							req := &pb.GenerateRequest{}
							if err := proto.Unmarshal(payload, req); err == nil {
								tokenCount := len(req.Tokenized.InputIds)
								originalText := req.Tokenized.OriginalText
								log.Printf("[Parser] Success: GenerateRequest (Tokens: %d) originalText: %s\n", tokenCount, originalText)
							} else {
								log.Printf("[Parser] Error unmarshalling GenerateRequest: %v\n", err)
							}
						} else {
							log.Println("Not valid /vllm.grpc.engine.VllmEngine/Generate")
						}
					}
				} else if s.enableHttpConvert {
					if contentType == "application/json" {
						switch grpcMethod {
						case "/v1/chat/completions":
							log.Println("[JSON Parser] Parsing JSON request /v1/chat/completions")
							jsonReq := &ChatCompletionRequest{}
							if err := json.Unmarshal(requestBodyBuffer, jsonReq); err != nil {
								log.Println("[JSON Parser] Error unmrshalling the JSON request", err)
								return err
							}
							protoReq := convertToGenerateRequest(*jsonReq)
							protoBytes, err := proto.Marshal(protoReq)
							if err != nil {
								log.Println("[Proto Parser] Error marshal the GenerateRequest proto message", err)
								return err
							}
							// gRPC 5-byte Prefix (Compression Flag + 4-byte Length)
							grpcFrame := make([]byte, 5)
							grpcFrame[0] = 0 // No compression
							binary.BigEndian.PutUint32(grpcFrame[1:], uint32(len(protoBytes)))
							// Change the requestBody to proto format
							requestBodyBuffer = append(grpcFrame, protoBytes...)
							contentTypeToSet = "application/grpc" // Set the contentType to gRPC.
						}
					} else { // http get request does not have any contentType set.
						// Assuming this is always the GetModelInfo
						protoBytes := []byte{}
						if err := proto.Unmarshal(protoBytes, &pb.GetModelInfoRequest{}); err != nil {
							log.Println("[Proto Parser] Error unmarshalling the GetModelInfoRequest proto message", err)
							return err
						}
						requestBodyBuffer = protoBytes
						contentTypeToSet = "application/grpc"
					}
				}

				if targetPodIP != "" {
					log.Printf("[Action] Setting Header %s=%s\n", metadata.DestinationEndpointKey, targetPodIP)
					headersToSet = []*configPb.HeaderValueOption{
						{
							Header: &configPb.HeaderValue{
								Key:      metadata.DestinationEndpointKey,
								RawValue: []byte(targetPodIP),
							},
							AppendAction: configPb.HeaderValueOption_APPEND_IF_EXISTS_OR_ADD,
						},
					}
				} else {
					log.Println("[Action] No Target IP determined.")
				}

				if s.enableHttpConvert && contentType == "application/json" {
					log.Printf("[Action] Setting Header %s=%s\n", "content-type", contentTypeToSet)
					additionalHeaders := []*configPb.HeaderValueOption{
						{
							Header: &configPb.HeaderValue{
								Key:      "content-type",
								RawValue: []byte(contentTypeToSet),
							},
							AppendAction: configPb.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
						},
						{
							Header: &configPb.HeaderValue{
								Key:      ":path",
								RawValue: []byte("/vllm.grpc.engine.VllmEngine/Generate"),
							},
							AppendAction: configPb.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
						},
						{
							Header: &configPb.HeaderValue{
								Key:      "te",
								RawValue: []byte("trailers"),
							},
							AppendAction: configPb.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
						},
						{
							Header: &configPb.HeaderValue{
								Key:      "grpc-timeout",
								RawValue: []byte("119919m"),
							},
							AppendAction: configPb.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
						},
					}
					headersToSet = append(headersToSet, additionalHeaders...)
				}

				// Send HeaderMutation Response
				resp = &extProcPb.ProcessingResponse{
					Response: &extProcPb.ProcessingResponse_RequestHeaders{
						RequestHeaders: &extProcPb.HeadersResponse{
							Response: &extProcPb.CommonResponse{
								Status: extProcPb.CommonResponse_CONTINUE,
								HeaderMutation: &extProcPb.HeaderMutation{
									SetHeaders: headersToSet,
								},
							},
						},
					},
				}
				log.Println("[Action] Sending Header Mutation Response")
				// Send the response immediately
				if err := stream.Send(resp); err != nil {
					log.Printf("[Error] Failed to send response: %v\n", err)
					return err
				}
				log.Println("[Action] Sent Body Response (Buffered for final send)")

				resp = &extProcPb.ProcessingResponse{
					Response: &extProcPb.ProcessingResponse_RequestBody{
						RequestBody: &extProcPb.BodyResponse{
							Response: &extProcPb.CommonResponse{
								BodyMutation: &extProcPb.BodyMutation{
									Mutation: &extProcPb.BodyMutation_StreamedResponse{
										StreamedResponse: &extProcPb.StreamedBodyResponse{
											Body:        requestBodyBuffer, // Echo back what we buffered
											EndOfStream: true,
										},
									},
								},
							},
						},
					},
				}
				requestBodyBuffer = nil
			}

		// ----------------------------------------------------------------
		// RESPONSE HEADERS PHASE
		// ----------------------------------------------------------------
		case *extProcPb.ProcessingRequest_ResponseHeaders:
			phase = "ResponseHeaders"
			log.Println(">>> [Phase: ResponseHeaders] Passing through...")
			// http response header
			//     1. non-streaming: https://screenshot.googleplex.com/7bdrLJGLYoVT3bQ
			//.    2. v1/models: https://screenshot.googleplex.com/98yrUmpPBWzbRYF
			//.    3. streaming: https://screenshot.googleplex.com/5yKduHXrqq46uJc
			// grp response header
			//    1.  non-streaming https://screenshot.googleplex.com/8h2ukbDz9mCRFpJ
			//.   2.  streaming: https://screenshot.googleplex.com/3usDASi6mdcv9SB
			//.   3.  getModelInfo: https://screenshot.googleplex.com/7RxmA8ResqVRLM9
			for _, h := range v.ResponseHeaders.Headers.Headers {
				log.Printf("    - Key=%q Value=%q RawValue=%q", h.Key, h.Value, h.RawValue)
			}

			var respHeadersToSet []*configPb.HeaderValueOption
			if s.enableHttpConvert && contentType == "application/grpc" {
				// Converting back from gRPC to json
				log.Printf("[Action] Setting Response Header %s=%s\n", "content-type", contentTypeToSet)
				additionalHeaders := []*configPb.HeaderValueOption{
					{
						Header: &configPb.HeaderValue{
							Key:      "content-type",
							RawValue: []byte("application/json"),
						},
						AppendAction: configPb.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
					},
					{
						Header: &configPb.HeaderValue{
							Key:      "content-length",
							RawValue: []byte("656"),
						},
						AppendAction: configPb.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
					},
				}
				respHeadersToSet = append(respHeadersToSet, additionalHeaders...)

				log.Println(">>> [Action] Preparing Response Header Mutation Response")
				// Send HeaderMutation Response
				resp = &extProcPb.ProcessingResponse{
					Response: &extProcPb.ProcessingResponse_ResponseHeaders{
						ResponseHeaders: &extProcPb.HeadersResponse{
							Response: &extProcPb.CommonResponse{
								Status: extProcPb.CommonResponse_CONTINUE,
								HeaderMutation: &extProcPb.HeaderMutation{
									SetHeaders: respHeadersToSet,
								},
							},
						},
					},
				}
			} else {
				resp = &extProcPb.ProcessingResponse{
					Response: &extProcPb.ProcessingResponse_ResponseHeaders{
						ResponseHeaders: &extProcPb.HeadersResponse{},
					},
				}
			}

		// ----------------------------------------------------------------
		// RESPONSE BODY PHASE
		// ----------------------------------------------------------------
		case *extProcPb.ProcessingRequest_ResponseBody:

			phase = "ResponseBody"
			log.Println(">>> [Phase: ResponseBody] Passing through...")
			responseBodyBuffer = append(responseBodyBuffer, v.ResponseBody.Body...)
			if s.enableHttpConvert {
				switch grpcMethod {
				// using http path because this is originally an http request.
				case "/v1/chat/completions":
					// In response we need to convert proto back to json.
					generateResp := &pb.GenerateResponse{}
					if payload, ok := parseGrpcPayload(v.ResponseBody.Body); ok {
						if err := proto.Unmarshal(payload, generateResp); err == nil {
							log.Println(">>> [Parser] Success: GenerateResponse")
						}
						// Convert back to json.
						responseBodyBuffer, err = json.Marshal(concevrtToGenerateResp(generateResp))
						if err != nil {
							log.Println(">>> [Parser] Fail: Convert GenerateResponse back to JSON")
							return err
						} else {
							log.Println(">>> [Parser] Success: Convert GenerateResponse back to JSON")
						}
					} else {
						log.Println("Not valid /vllm.grpc.engine.VllmEngine/Generate")
					}
				case "/v1/models":
					log.Println(">> [Parser] parsing GetModelInfo")
					protoGetModelInfoRes := &pb.GetModelInfoResponse{}
					if payload, ok := parseGrpcPayload(v.ResponseBody.Body); ok {
						if err := proto.Unmarshal(payload, protoGetModelInfoRes); err == nil {
							log.Println(">>> [Parser] Success: GetModelInfo")
						}
						responseBodyBuffer, err = json.Marshal(convertToModelInfoResponse(protoGetModelInfoRes))
						if err != nil {
							log.Println(">>> [Parser] Fail: Convert GetModelInfoRes back to JSON")
							return err
						} else {
							log.Println(">>> [Parser] Success: Convert GetModelInfoRes back to JSON")
						}
					} else {
						log.Println("Not valid /vllm.grpc.engine.VllmEngine/GetModelInfo")
					}
				}
			}
			resps := generateResponseBodyResponses(responseBodyBuffer, v.ResponseBody.GetEndOfStream())
			log.Println("Resps length", len(resps))
			if len(resps) > 0 {
				resp = resps[0]
			}
			responseBodyResps = append(responseBodyResps, resps...)
			if !v.ResponseBody.EndOfStream {
				log.Println(" [Phase: ResponseBody] non end of stream")
				// continue
			} else {
				log.Println(" [Phase: ResponseBody] End of stream")
			}

		// ----------------------------------------------------------------
		// RESPONSE TRAILERS PHASE
		// ----------------------------------------------------------------
		case *extProcPb.ProcessingRequest_ResponseTrailers:
			phase = "ResponseTrailers"
			log.Println(">>> [Phase: ResponseTrailers] Passing through...")
			resp = &extProcPb.ProcessingResponse{
				Response: &extProcPb.ProcessingResponse_ResponseTrailers{
					ResponseTrailers: &extProcPb.TrailersResponse{},
				},
			}

		default:
			log.Printf(">>> [Phase: Unknown/Default] %T\n", req.Request)
			resp = &extProcPb.ProcessingResponse{
				Response: &extProcPb.ProcessingResponse_ResponseTrailers{
					ResponseTrailers: &extProcPb.TrailersResponse{},
				},
			}
		}

		if resp.Response == nil {
			log.Printf("[Warning] Nothing to send for phase %s\n", phase)
			continue
		}

		log.Println("[Action] Sending response")
		if len(responseBodyResps) != 0 {
			// responseBodyResps is only populated when processing ResponseBody
			for i, resp := range responseBodyResps {
				if err := stream.Send(resp); err != nil {
					log.Printf("[Error] Failed to send response #%d: %v\n", i, err)
					return err
				}
			}
			// Clear (not sure if this is needed)
			responseBodyResps = []*extProcPb.ProcessingResponse{}
		} else {
			if err := stream.Send(resp); err != nil {
				log.Printf("[Error] Failed to send response: %v\n", err)
				return err
			}
		}
	}
}

func main() {
	// Parse Flags
	targetIP := flag.String("target-ip", "", "The target Pod IP to route requests to (overrides dynamic logic if set)")
	enableHttpConvert := flag.Bool("enable-http-convert", false, "if set, the http request will be converted to grpc and send to the model server.")
	flag.Parse()

	if *targetIP != "" {
		log.Printf("Starting ExtProc with STATIC Target IP: %s\n", *targetIP)
	} else {
		log.Println("Starting ExtProc with DYNAMIC Routing Logic")
	}
	log.Println("enableHttpConvert is set to", *enableHttpConvert)

	var wg sync.WaitGroup

	// ----------------------------------------
	// Service 1: Ext Proc (Port 9002)
	// ----------------------------------------
	wg.Add(1)
	go func() {
		defer wg.Done()
		lis, err := net.Listen("tcp", ":9002")
		if err != nil {
			log.Fatalf("failed to listen on 9002: %v", err)
		}
		s := grpc.NewServer()
		extProcPb.RegisterExternalProcessorServer(s, &extProcServer{
			targetIP:          *targetIP,
			enableHttpConvert: *enableHttpConvert,
		})

		log.Println("ExtProc listening on :9002")
		if err := s.Serve(lis); err != nil {
			log.Printf("ExtProc server error: %v", err)
		}
	}()

	// ----------------------------------------
	// Service 2: gRPC Health Check (Port 9003)
	// ----------------------------------------
	wg.Add(1)
	go func() {
		defer wg.Done()
		lis, err := net.Listen("tcp", ":9003")
		if err != nil {
			log.Fatalf("failed to listen on 9003: %v", err)
		}

		s := grpc.NewServer()
		healthServer := health.NewServer()

		healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
		healthServer.SetServingStatus("inference-extension", grpc_health_v1.HealthCheckResponse_SERVING)

		grpc_health_v1.RegisterHealthServer(s, healthServer)

		log.Println("gRPC Health listening on :9003")
		if err := s.Serve(lis); err != nil {
			log.Printf("Health server error: %v", err)
		}
	}()

	// ----------------------------------------
	// Service 3: Metrics / HTTP (Port 9090)
	// ----------------------------------------
	wg.Add(1)
	go func() {
		defer wg.Done()
		mux := http.NewServeMux()
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		})

		server := &http.Server{
			Addr:    ":9090",
			Handler: mux,
		}

		log.Println("📊 Metrics/HTTP listening on :9090")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Metrics server error: %v", err)
		}
	}()

	wg.Wait()
}

const (
	// Certain envoy implementations set a max limit of 64Kb per streamed chunk, intentionally setting this lower for a safe margin.
	bodyByteLimit = 62000
)

func generateRequestBodyResponses(requestBodyBytes []byte) []*extProcPb.ProcessingResponse {
	commonResponses := buildCommonResponses(requestBodyBytes, bodyByteLimit, true)
	responses := []*extProcPb.ProcessingResponse{}
	for _, commonResp := range commonResponses {
		resp := &extProcPb.ProcessingResponse{
			Response: &extProcPb.ProcessingResponse_RequestBody{
				RequestBody: &extProcPb.BodyResponse{
					Response: commonResp,
				},
			},
		}
		responses = append(responses, resp)
	}
	return responses
}

func generateResponseBodyResponses(responseBodyBytes []byte, setEoS bool) []*extProcPb.ProcessingResponse {
	commonResponses := buildCommonResponses(responseBodyBytes, bodyByteLimit, setEoS)
	responses := []*extProcPb.ProcessingResponse{}
	for _, commonResp := range commonResponses {
		resp := &extProcPb.ProcessingResponse{
			Response: &extProcPb.ProcessingResponse_ResponseBody{
				ResponseBody: &extProcPb.BodyResponse{
					Response: commonResp,
				},
			},
		}
		responses = append(responses, resp)
	}
	return responses
}

func buildCommonResponses(bodyBytes []byte, byteLimit int, setEos bool) []*extProcPb.CommonResponse {
	responses := []*extProcPb.CommonResponse{}
	startingIndex := 0
	bodyLen := len(bodyBytes)

	if bodyLen == 0 {
		return []*extProcPb.CommonResponse{
			{
				BodyMutation: &extProcPb.BodyMutation{
					Mutation: &extProcPb.BodyMutation_StreamedResponse{
						StreamedResponse: &extProcPb.StreamedBodyResponse{
							Body:        bodyBytes,
							EndOfStream: setEos,
						},
					},
				},
			},
		}
	}

	for startingIndex < bodyLen {
		eos := false
		len := min(bodyLen-startingIndex, byteLimit)
		chunk := bodyBytes[startingIndex : len+startingIndex]
		if setEos && len+startingIndex >= bodyLen {
			eos = true
		}

		commonResp := &extProcPb.CommonResponse{
			BodyMutation: &extProcPb.BodyMutation{
				Mutation: &extProcPb.BodyMutation_StreamedResponse{
					StreamedResponse: &extProcPb.StreamedBodyResponse{
						Body:        chunk,
						EndOfStream: eos,
					},
				},
			},
		}
		responses = append(responses, commonResp)
		startingIndex += len
	}

	return responses
}

type ChatCompletionRequest struct {
	Model               string    `json:"model"`
	Messages            []Message `json:"messages"`
	MaxCompletionTokens int32     `json:"max_completion_tokens"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func convertToGenerateRequest(request ChatCompletionRequest) *pb.GenerateRequest {
	tokenizer := NewTokenizer()
	maxToken := int32(50)
	return &pb.GenerateRequest{
		RequestId: "fake-request-id",
		Tokenized: &pb.TokenizedInput{
			OriginalText: request.Messages[0].Content,
			InputIds:     tokenizer.Tokenize(request.Messages[0].Content),
		},
		SamplingParams: &pb.SamplingParams{
			Temperature: 0.7,
			MaxTokens:   &maxToken,
		},
		Stream: false,
	}
}

type ChatCompletionResponse struct {
	Message      string   `json:"message"`
	OutputTokens []uint32 `json:"output_tokens"`
	FinishReason string   `json:"finished_reason"`
}

type ModelInfoResponse struct {
	Model string `json:model`
}

func concevrtToGenerateResp(resp *pb.GenerateResponse) ChatCompletionResponse {
	if resp != nil {
		log.Printf("[Converting] response is %s\n", *resp)
	} else {
		log.Println("[Converting] response is nil")
	}
	var outputTokens []uint32
	var finishedReason string
	if resp.GetComplete() != nil {
		log.Println("[Converting] resp.GetComplete() is not nil")
		outputTokens = resp.GetComplete().GetOutputIds()
		finishedReason = resp.GetComplete().FinishReason
	} else if resp.GetChunk() != nil {
		log.Println("[Converting] resp.GetChunk() is not nil")
		outputTokens = resp.GetChunk().GetTokenIds()
	}
	chatResp := ChatCompletionResponse{
		OutputTokens: outputTokens,
		FinishReason: finishedReason,
	}
	return chatResp
}

func convertToModelInfoResponse(resp *pb.GetModelInfoResponse) ModelInfoResponse {
	if resp != nil {
		log.Printf("[Converting] modelInfo response is %s\n", *resp)
	} else {
		log.Println("[Converting] modelInfo response is nil")
	}
	return ModelInfoResponse{
		Model: resp.ModelPath,
	}
}

type Tokenizer struct {
	vocab        map[uint32]string
	reverseVocab map[string]uint32
}

func NewTokenizer() *Tokenizer {
	vocab := map[uint32]string{
		1:  "Hello",
		2:  ",",
		3:  " ",
		4:  "world",
		5:  "!",
		10: "Tell",
		11: " ",
		12: "me",
		13: " ",
		14: "a",
		15: " ",
		16: "story",
		17: ".",
	}
	reverseVocab := map[string]uint32{}
	for k, v := range vocab {
		reverseVocab[v] = k
	}
	return &Tokenizer{
		vocab:        vocab,
		reverseVocab: reverseVocab,
	}
}

func (t *Tokenizer) Tokenize(text string) []uint32 {
	voca := strings.Split(text, " ")
	tokens := make([]uint32, 0, len(voca))
	for _, v := range voca {
		if token, ok := t.reverseVocab[v]; ok {
			tokens = append(tokens, token)
		} else {
			tokens = append(tokens, 0)
		}
	}
	return tokens
}

func (t *Tokenizer) Detokenize(ids []uint32) string {
	var sb strings.Builder
	for _, id := range ids {
		if val, ok := t.vocab[id]; ok {
			sb.WriteString(val)
		} else {
			// Fallback for unknown tokens
			sb.WriteString(fmt.Sprintf("<%d>", id))
		}
	}
	return sb.String()
}
