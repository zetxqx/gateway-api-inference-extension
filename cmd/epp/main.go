package main

import (
	"encoding/binary"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"sync"

	configPb "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
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

type extProcServer struct {
	targetIP string
}

func (s *extProcServer) Process(stream extProcPb.ExternalProcessor_ProcessServer) error {
	ctx := stream.Context()

	// State variables per stream
	var grpcMethod string
	var requestBodyBuffer []byte
	var responseBodyBuffer []byte

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

		resp := &extProcPb.ProcessingResponse{}
		phase := "Unknown"
		log.Println("[Info] Back to swtich case")

		switch v := req.Request.(type) {

		// ----------------------------------------------------------------
		// HEADER PHASE
		// ----------------------------------------------------------------
		case *extProcPb.ProcessingRequest_RequestHeaders:
			phase = "RequestHeaders"
			log.Println(">>> [Phase: RequestHeaders] <<<")
			log.Printf(" Printing all %d headers:", len(v.RequestHeaders.Headers.Headers))
			for _, h := range v.RequestHeaders.Headers.Headers {
				log.Printf("    - Key=%q Value=%q RawValue=%q", h.Key, h.Value, h.RawValue)
				if h.Key == ":path" {
					grpcMethod = string(h.RawValue)
				}
			}

			log.Printf("    Method Found: %s\n", grpcMethod)
			continue

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
					if payload, ok := parseGrpcPayload(requestBodyBuffer); ok {
						req := &pb.GenerateRequest{}
						if err := proto.Unmarshal(payload, req); err == nil {
							tokenCount := len(req.Tokenized.InputIds)
							originalText := req.Tokenized.OriginalText
							log.Printf("[Parser] Success: GenerateRequest (Tokens: %d) originalText: %s\n", tokenCount, originalText)
						} else {
							log.Printf("[Parser] Error unmarshalling GenerateRequest: %v\n", err)
						}
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
			resp = &extProcPb.ProcessingResponse{
				Response: &extProcPb.ProcessingResponse_ResponseHeaders{
					ResponseHeaders: &extProcPb.HeadersResponse{},
				},
			}

		// ----------------------------------------------------------------
		// RESPONSE BODY PHASE
		// ----------------------------------------------------------------
		case *extProcPb.ProcessingRequest_ResponseBody:

			phase = "ResponseBody"
			log.Println(">>> [Phase: ResponseBody] Passing through...")
			responseBodyBuffer = append(requestBodyBuffer, v.ResponseBody.Body...)

			resps := generateResponseBodyResponses(responseBodyBuffer, false)
			log.Println("Resps length", len(resps))
			if len(resps) > 0 {
				resp = resps[0]
			}
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

		log.Println("[Action] Sending resposne")
		if err := stream.Send(resp); err != nil {
			log.Printf("[Error] Failed to send response: %v\n", err)
			return err
		}
	}
}

// ==========================================
// 2. Main Entry Point
// ==========================================

func main() {
	// Parse Flags
	targetIP := flag.String("target-ip", "", "The target Pod IP to route requests to (overrides dynamic logic if set)")
	flag.Parse()

	if *targetIP != "" {
		log.Printf("Starting ExtProc with STATIC Target IP: %s\n", *targetIP)
	} else {
		log.Println("Starting ExtProc with DYNAMIC Routing Logic")
	}

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
			targetIP: *targetIP,
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
