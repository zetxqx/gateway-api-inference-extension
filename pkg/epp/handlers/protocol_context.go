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

package handlers

import (
	extProcPb "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/go-logr/logr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	logutil "sigs.k8s.io/gateway-api-inference-extension/pkg/common/observability/logging"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/metrics"
)

// ProtocolContext manages the state of the Envoy External Processing protocol.
type ProtocolContext struct {
	RequestState         StreamRequestState
	modelServerStreaming bool

	reqHeaderResp  *extProcPb.ProcessingResponse
	reqBodyResp    []*extProcPb.ProcessingResponse
	reqTrailerResp *extProcPb.ProcessingResponse

	respHeaderResp  *extProcPb.ProcessingResponse
	respBodyResp    []*extProcPb.ProcessingResponse
	respTrailerResp *extProcPb.ProcessingResponse
}

type StreamRequestState int

const (
	RequestReceived                  StreamRequestState = 0
	HeaderRequestResponseComplete    StreamRequestState = 1
	BodyRequestResponsesComplete     StreamRequestState = 2
	TrailerRequestResponsesComplete  StreamRequestState = 3
	ResponseReceived                 StreamRequestState = 4
	HeaderResponseResponseComplete   StreamRequestState = 5
	BodyResponseResponsesComplete    StreamRequestState = 6
	TrailerResponseResponsesComplete StreamRequestState = 7
)

// updateStateAndSendIfNeeded checks state and can send mutiple responses in a single pass, but only if ordered properly.
// Order of requests matter in FULL_DUPLEX_STREAMING. For both request and response, the order of response sent back MUST be: Header->Body->Trailer, with trailer being optional.
func (p *ProtocolContext) updateStateAndSendIfNeeded(srv extProcPb.ExternalProcessor_ProcessServer, logger logr.Logger, reqCtx *RequestContext) error {
	loggerTrace := logger.V(logutil.TRACE)
	// No switch statement as we could send multiple responses in one pass.
	if p.RequestState == RequestReceived && p.reqHeaderResp != nil {
		loggerTrace.Info("Sending request header response", "obj", p.reqHeaderResp)
		if err := srv.Send(p.reqHeaderResp); err != nil {
			logger.V(logutil.DEFAULT).Error(err, "error sending response")
			return status.Errorf(codes.Unknown, "failed to send response back to Envoy: %v", err)
		}
		p.RequestState = HeaderRequestResponseComplete
	}
	if p.RequestState == HeaderRequestResponseComplete && p.reqBodyResp != nil && len(p.reqBodyResp) > 0 {
		loggerTrace.Info("Sending request body response(s)")

		for _, response := range p.reqBodyResp {
			if err := srv.Send(response); err != nil {
				return status.Errorf(codes.Unknown, "failed to send response back to Envoy: %v", err)
			}
		}
		p.RequestState = BodyRequestResponsesComplete
		metrics.IncRunningRequests(reqCtx.IncomingModelName)
		reqCtx.RequestRunning = true
		// Dump the response so a new stream message can begin
		p.reqBodyResp = nil
	}
	if p.RequestState == BodyRequestResponsesComplete && p.reqTrailerResp != nil {
		// Trailers in requests are not guaranteed
		if err := srv.Send(p.reqTrailerResp); err != nil {
			return status.Errorf(codes.Unknown, "failed to send response back to Envoy: %v", err)
		}
	}
	if p.RequestState == ResponseReceived && p.respHeaderResp != nil {
		loggerTrace.Info("Sending response header response", "obj", p.respHeaderResp)
		if err := srv.Send(p.respHeaderResp); err != nil {
			return status.Errorf(codes.Unknown, "failed to send response back to Envoy: %v", err)
		}
		p.RequestState = HeaderResponseResponseComplete
	}
	if p.RequestState == HeaderResponseResponseComplete && p.respBodyResp != nil && len(p.respBodyResp) > 0 {
		loggerTrace.Info("Sending response body response(s)")
		for _, response := range p.respBodyResp {
			if err := srv.Send(response); err != nil {
				return status.Errorf(codes.Unknown, "failed to send response back to Envoy: %v", err)
			}

			body := response.Response.(*extProcPb.ProcessingResponse_ResponseBody)
			if body.ResponseBody.Response.GetBodyMutation().GetStreamedResponse().GetEndOfStream() {
				p.RequestState = BodyResponseResponsesComplete
			}
		}
		// Dump the response so a new stream message can begin
		p.respBodyResp = nil
	}
	if p.RequestState == BodyResponseResponsesComplete && p.respTrailerResp != nil {
		// Trailers in requests are not guaranteed
		if err := srv.Send(p.respTrailerResp); err != nil {
			return status.Errorf(codes.Unknown, "failed to send response back to Envoy: %v", err)
		}
	}
	return nil
}
