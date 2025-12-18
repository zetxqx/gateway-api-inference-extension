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
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	// Your internal packages
	pb "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/grpc/gen"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/metadata"
)

// Helper: Parse gRPC 5-byte framing
func parseGrpcPayload(data []byte) ([]byte, bool) {
	if len(data) > 5 {
		msgLen := binary.BigEndian.Uint32(data[1:5])
		if uint32(len(data)) >= 5+msgLen {
			return data[5 : 5+msgLen], true
		}
	}
	return nil, false
}

// ==========================================
// 1. External Processor Server
// ==========================================

type extProcServer struct {
	targetIP string
}

func (s *extProcServer) Process(stream extProcPb.ExternalProcessor_ProcessServer) error {
	ctx := stream.Context()

	// State variables per stream
	var grpcMethod string
	var requestBodyBuffer []byte

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

		switch v := req.Request.(type) {

		// ----------------------------------------------------------------
		// HEADER PHASE
		// ----------------------------------------------------------------
		case *extProcPb.ProcessingRequest_RequestHeaders:
			phase = "RequestHeaders"
			log.Println(">>> [Phase: RequestHeaders] <<<")
			log.Printf("    Printing all %d headers:", len(v.RequestHeaders.Headers.Headers))
			for _, h := range v.RequestHeaders.Headers.Headers {
				log.Printf("    - Key=%q Value=%q RawValue=%q", h.Key, h.Value, h.RawValue)
				if h.Key == ":path" {
					grpcMethod = string(h.RawValue)
				}
			}

			log.Printf("    Method Found: %s\n", grpcMethod)

			// STOP_AND_BUFFER: Hold the headers! Do not send them upstream yet.
			// resp = &extProcPb.ProcessingResponse{
			// 	Response: &extProcPb.ProcessingResponse_RequestHeaders{
			// 		RequestHeaders: &extProcPb.HeadersResponse{
			// 			Response: &extProcPb.CommonResponse{
			// 				Status: extProcPb.CommonResponse_STOP_AND_BUFFER,
			// 			},
			// 		},
			// 	},
			// }
			resp = &extProcPb.ProcessingResponse{
				Response: &extProcPb.ProcessingResponse_RequestHeaders{
					RequestHeaders: &extProcPb.HeadersResponse{
						Response: &extProcPb.CommonResponse{
							// In a real scheduler, you might delay this if you need body for scheduling
							// But for "simplest" demo, we just verify we got the headers
						},
					},
				},
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
				resp = &extProcPb.ProcessingResponse{
					Response: &extProcPb.ProcessingResponse_RequestBody{
						RequestBody: &extProcPb.BodyResponse{
							Response: &extProcPb.CommonResponse{
								Status: extProcPb.CommonResponse_CONTINUE,
							},
						},
					},
				}
			} else {
				// End of Stream: Process Logic
				log.Println("    [Processing Logic] Full body received. Decoding...")

				var headersToSet []*configPb.HeaderValueOption
				targetPodIP := ""

				// 1. Check Flag Override
				if s.targetIP != "" {
					targetPodIP = s.targetIP
					log.Printf("    [Decision] Using Configured Flag Target IP: %s\n", targetPodIP)
				}

				// 2. Dynamic Routing (if no flag)
				switch grpcMethod {
				case "/vllm.grpc.engine.VllmEngine/GetModelInfo":
					if payload, ok := parseGrpcPayload(requestBodyBuffer); ok {
						req := &pb.GetModelInfoRequest{}
						if err := proto.Unmarshal(payload, req); err == nil {
							log.Println("    [Parser] Success: GetModelInfoRequest")
						}
					}

				case "/vllm.grpc.engine.VllmEngine/Generate":
					if payload, ok := parseGrpcPayload(requestBodyBuffer); ok {
						req := &pb.GenerateRequest{}
						if err := proto.Unmarshal(payload, req); err == nil {
							tokenCount := len(req.Tokenized.InputIds)
							log.Printf("    [Parser] Success: GenerateRequest (Tokens: %d)\n", tokenCount)
						} else {
							log.Printf("    [Parser] Error unmarshalling GenerateRequest: %v\n", err)
						}
					}
				}

				// 3. Set Headers
				if targetPodIP != "" {
					log.Printf("    [Action] Setting Header %s=%s\n", metadata.DestinationEndpointKey, targetPodIP)
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
					log.Println("    [Action] No Target IP determined.")
				}

				// 4. Send Response
				resp = &extProcPb.ProcessingResponse{
					Response: &extProcPb.ProcessingResponse_RequestBody{
						RequestBody: &extProcPb.BodyResponse{
							Response: &extProcPb.CommonResponse{
								Status: extProcPb.CommonResponse_CONTINUE,
								HeaderMutation: &extProcPb.HeaderMutation{
									SetHeaders: headersToSet,
								},
							},
						},
					},
				}
				log.Println("    [Action] Sending Body Response")

				// Send the response immediately
				if err := stream.Send(resp); err != nil {
					log.Printf("❌ [Error] Failed to send response: %v\n", err)
					return err
				}
				log.Println("    [Action] Sent Body Response")

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

				// Clear buffer
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
					ResponseHeaders: &extProcPb.HeadersResponse{
						Response: &extProcPb.CommonResponse{
							Status: extProcPb.CommonResponse_CONTINUE,
						},
					},
				},
			}

		// ----------------------------------------------------------------
		// RESPONSE BODY PHASE
		// ----------------------------------------------------------------
		case *extProcPb.ProcessingRequest_ResponseBody:
			phase = "ResponseBody"
			resp = &extProcPb.ProcessingResponse{
				Response: &extProcPb.ProcessingResponse_ResponseBody{
					ResponseBody: &extProcPb.BodyResponse{
						Response: &extProcPb.CommonResponse{
							Status: extProcPb.CommonResponse_CONTINUE,
						},
					},
				},
			}

		default:
			log.Printf(">>> [Phase: Unknown/Default] %T\n", req.Request)
			resp = &extProcPb.ProcessingResponse{
				Response: &extProcPb.ProcessingResponse_ImmediateResponse{
					ImmediateResponse: &extProcPb.ImmediateResponse{
						Status: &typev3.HttpStatus{Code: typev3.StatusCode_OK},
					},
				},
			}
		}

		if resp.Response == nil {
			log.Printf("⚠️  [Warning] Nothing to send for phase %s\n", phase)
			continue
		}

		if err := stream.Send(resp); err != nil {
			log.Printf("❌ [Error] Failed to send response: %v\n", err)
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
		log.Printf("🚀 Starting ExtProc with STATIC Target IP: %s\n", *targetIP)
	} else {
		log.Println("🚀 Starting ExtProc with DYNAMIC Routing Logic")
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

		log.Println("✅ ExtProc listening on :9002")
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

		log.Println("❤️  gRPC Health listening on :9003")
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
