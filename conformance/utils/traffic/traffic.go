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

package traffic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"regexp"
	"strings"
	"testing"
	"time"

	gwconfig "sigs.k8s.io/gateway-api/conformance/utils/config"
	gwhttp "sigs.k8s.io/gateway-api/conformance/utils/http"
	"sigs.k8s.io/gateway-api/conformance/utils/roundtripper"
	"sigs.k8s.io/gateway-api/conformance/utils/tlog"
)

// BuildExpectedHTTPResponse constructs a gwhttp.ExpectedResponse for common test scenarios.
// For 200 OK responses, it sets up ExpectedRequest to check Host and Path.
// For other status codes (like 404), ExpectedRequest is nil as detailed backend checks are usually skipped by CompareRequest.
func BuildExpectedHTTPResponse(
	requestHost string,
	requestPath string,
	expectedStatusCode int,
	backendName string,
	backendNamespace string,
) gwhttp.ExpectedResponse {
	return BuildExpectedHTTPResponseWithHeaders(requestHost, requestPath, expectedStatusCode, backendName, backendNamespace, nil, "GET")
}

func BuildExpectedHTTPResponseWithHeaders(
	requestHost string,
	requestPath string,
	expectedStatusCode int,
	backendName string,
	backendNamespace string,
	headers map[string]string,
	method string,
) gwhttp.ExpectedResponse {
	resp := gwhttp.ExpectedResponse{
		Request: gwhttp.Request{
			Host:    requestHost,
			Path:    requestPath,
			Method:  method,
			Headers: headers,
		},
		Response: gwhttp.Response{
			StatusCode: expectedStatusCode,
		},
		Backend:   backendName,
		Namespace: backendNamespace,
	}

	if expectedStatusCode == http.StatusOK {
		resp.ExpectedRequest = &gwhttp.ExpectedRequest{
			Request: gwhttp.Request{
				Host:    requestHost,
				Path:    requestPath,
				Headers: headers,
				Method:  method,
			},
		}
	}
	return resp
}

// MakeRequestAndExpectSuccess is a helper function that builds an expected success (200 OK) response
// and then calls MakeRequestAndExpectEventuallyConsistentResponse.
func MakeRequestAndExpectSuccess(
	t *testing.T,
	r roundtripper.RoundTripper,
	timeoutConfig gwconfig.TimeoutConfig,
	gatewayAddress string,
	requestHost string,
	requestPath string,
	backendName string,
	backendNamespace string,
) {
	t.Helper()
	expectedResponse := BuildExpectedHTTPResponse(
		requestHost,
		requestPath,
		http.StatusOK,
		backendName,
		backendNamespace,
	)
	gwhttp.MakeRequestAndExpectEventuallyConsistentResponse(t, r, timeoutConfig, gatewayAddress, expectedResponse)
}

// MakeRequestAndExpectNotFound is a helper function that builds an expected not found (404) response
// and then calls MakeRequestAndExpectEventuallyConsistentResponse.
func MakeRequestAndExpectNotFound(
	t *testing.T,
	r roundtripper.RoundTripper,
	timeoutConfig gwconfig.TimeoutConfig,
	gatewayAddress string,
	requestHost string,
	requestPath string,
) {
	t.Helper()
	expectedResponse := BuildExpectedHTTPResponse(
		requestHost,
		requestPath,
		http.StatusNotFound,
		"", // Backend name not relevant for 404
		"", // Backend namespace not relevant for 404
	)
	gwhttp.MakeRequestAndExpectEventuallyConsistentResponse(t, r, timeoutConfig, gatewayAddress, expectedResponse)
}

// MakeRequestAndExpectSuccessV2 is a helper function that builds an expected success (200 OK) response.
// And then make the request and waiting for the response to be expected.
func MakeRequestAndExpectSuccessV2(
	t *testing.T,
	r roundtripper.RoundTripper,
	timeoutConfig gwconfig.TimeoutConfig,
	gatewayAddress string,
	requestHost string,
	requestPath string,
	backendName string,
	backendNamespace string,
	headers map[string]string,
	reqBody string,
	method string,
) {
	t.Helper()
	makeRequestAndExpectRequestInternal(
		t,
		r,
		timeoutConfig,
		gatewayAddress,
		requestHost,
		requestPath,
		backendName,
		backendNamespace,
		headers,
		reqBody,
		method,
		http.StatusOK,
	)
}

// MakeRequestAndExpectTooManyRequest is a helper function that builds an expected too many request (429) response.
// And then make the request and waiting for the response to be expected.
func MakeRequestAndExpectTooManyRequest(
	t *testing.T,
	r roundtripper.RoundTripper,
	timeoutConfig gwconfig.TimeoutConfig,
	gatewayAddress string,
	requestHost string,
	requestPath string,
	backendName string,
	backendNamespace string,
	headers map[string]string,
	reqBody string,
	method string,
) {
	t.Helper()
	makeRequestAndExpectRequestInternal(
		t,
		r,
		timeoutConfig,
		gatewayAddress,
		requestHost,
		requestPath,
		backendName,
		backendNamespace,
		headers,
		reqBody,
		method,
		http.StatusTooManyRequests,
	)
}

// MakeRequestAndExpectBadRequest is a helper function that builds an expected bad request (400) response.
// And then make the request and waiting for the response to be expected.
func MakeRequestAndExpectBadRequest(
	t *testing.T,
	r roundtripper.RoundTripper,
	timeoutConfig gwconfig.TimeoutConfig,
	gatewayAddress string,
	requestHost string,
	requestPath string,
	backendName string,
	backendNamespace string,
	headers map[string]string,
	reqBody string,
	method string,
) {
	t.Helper()
	makeRequestAndExpectRequestInternal(
		t,
		r,
		timeoutConfig,
		gatewayAddress,
		requestHost,
		requestPath,
		backendName,
		backendNamespace,
		headers,
		reqBody,
		method,
		http.StatusBadRequest,
	)
}

// MakeRequestAndExpectBadRequest is a helper function that builds an expected not found (404) response.
// And then make the request and waiting for the response to be expected.
func MakeRequestAndExpectNotFoundV2(
	t *testing.T,
	r roundtripper.RoundTripper,
	timeoutConfig gwconfig.TimeoutConfig,
	gatewayAddress string,
	requestHost string,
	requestPath string,
	backendName string,
	backendNamespace string,
	headers map[string]string,
	reqBody string,
	method string,
) {
	t.Helper()
	makeRequestAndExpectRequestInternal(
		t,
		r,
		timeoutConfig,
		gatewayAddress,
		requestHost,
		requestPath,
		backendName,
		backendNamespace,
		headers,
		reqBody,
		method,
		http.StatusNotFound,
	)
}

func makeRequestAndExpectRequestInternal(t *testing.T,
	r roundtripper.RoundTripper,
	timeoutConfig gwconfig.TimeoutConfig,
	gatewayAddress string,
	requestHost string,
	requestPath string,
	backendName string,
	backendNamespace string,
	headers map[string]string,
	reqBody string,
	method string,
	expectedHttpCode int,
) {
	t.Helper()
	expectedResponse := BuildExpectedHTTPResponseWithHeaders(
		requestHost,
		requestPath,
		expectedHttpCode,
		backendName,
		backendNamespace,
		headers,
		method,
	)
	waitForConvergeToExpected(t, r, timeoutConfig, gatewayAddress, reqBody, expectedResponse)
}

// TODO: replace the following method when sigs.k8s.io/gateway-api/conformance/utils/roundtripper is able to send request with body.
func waitForConvergeToExpected(
	t *testing.T,
	r roundtripper.RoundTripper,
	timeoutConfig gwconfig.TimeoutConfig,
	gatewayAddress string,
	requestBody string,
	expectedResponse gwhttp.ExpectedResponse,
) {
	gwhttp.AwaitConvergence(t, timeoutConfig.RequiredConsecutiveSuccesses, timeoutConfig.MaxTimeToConsistency, func(elapsed time.Duration) bool {
		req := gwhttp.MakeRequest(t, &expectedResponse, gatewayAddress, "HTTP", "http")
		request := &RequestWithBody{Request: req, Body: strings.NewReader(requestBody)}
		defaultRoundTripper, ok := r.(*roundtripper.DefaultRoundTripper)
		if !ok {
			t.Fatalf("Unsupported RoundTripper type: %T", r)
		}
		cReq, cRes, err := makeCallRoundTripper(defaultRoundTripper, request)
		if err != nil {
			tlog.Logf(t, "Request failed, not ready yet: %v (after %v)", err.Error(), elapsed)
			return false
		}

		if err := gwhttp.CompareRequest(t, &request.Request, cReq, cRes, expectedResponse); err != nil {
			tlog.Logf(t, "Response expectation failed for request: %+v  not ready yet: %v (after %v)", request.Request, err, elapsed)
			return false
		}

		return true
	})
	tlog.Logf(t, "Request passed")
}

// RequestWithBody extends roundtripper.Request to include a request body.
type RequestWithBody struct {
	roundtripper.Request
	Body io.Reader
}

// makeCallRoundTripper executes an HTTP request using the provided RoundTripper and captures the request and response.
func makeCallRoundTripper(rt *roundtripper.DefaultRoundTripper, request *RequestWithBody) (*roundtripper.CapturedRequest, *roundtripper.CapturedResponse, error) {
	client := &http.Client{}

	if request.UnfollowRedirect {
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	client.Transport = &http.Transport{
		DialContext: rt.CustomDialContext,
		// We disable keep-alives so that we don't leak established TCP connections.
		// Leaking TCP connections is bad because we could eventually hit the
		// threshold of maximum number of open TCP connections to a specific
		// destination. Keep-alives are not presently utilized so disabling this has
		// no adverse affect.
		//
		// Ref. https://github.com/kubernetes-sigs/gateway-api/issues/2357
		DisableKeepAlives: true,
	}

	method := "GET"
	if request.Method != "" {
		method = request.Method
	}
	ctx, cancel := context.WithTimeout(context.Background(), rt.TimeoutConfig.RequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, request.URL.String(), request.Body)
	if err != nil {
		return nil, nil, err
	}

	if request.Host != "" {
		req.Host = request.Host
	}

	if request.Headers != nil {
		for name, value := range request.Headers {
			req.Header.Set(name, value[0])
		}
	}

	if rt.Debug {
		var dump []byte
		dump, err = httputil.DumpRequestOut(req, true)
		if err != nil {
			return nil, nil, err
		}

		tlog.Logf(request.T, "Sending Request:\n%s\n\n", formatDump(dump, "< "))
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if rt.Debug {
		var dump []byte
		dump, err = httputil.DumpResponse(resp, true)
		if err != nil {
			return nil, nil, err
		}

		tlog.Logf(request.T, "Received Response:\n%s\n\n", formatDump(dump, "< "))
	}

	cReq := &roundtripper.CapturedRequest{}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	// we cannot assume the response is JSON
	if resp.Header.Get("Content-type") == "application/json" {
		err = json.Unmarshal(body, cReq)
		if err != nil {
			return nil, nil, fmt.Errorf("unexpected error reading response: %w", err)
		}
	} else {
		cReq.Method = method // assume it made the right request if the service being called isn't echoing
	}

	cRes := &roundtripper.CapturedResponse{
		StatusCode:    resp.StatusCode,
		ContentLength: resp.ContentLength,
		Protocol:      resp.Proto,
		Headers:       resp.Header,
	}

	if resp.TLS != nil {
		cRes.PeerCertificates = resp.TLS.PeerCertificates
	}

	if roundtripper.IsRedirect(resp.StatusCode) {
		redirectURL, err := resp.Location()
		if err != nil {
			return nil, nil, err
		}
		cRes.RedirectRequest = &roundtripper.RedirectRequest{
			Scheme: redirectURL.Scheme,
			Host:   redirectURL.Hostname(),
			Port:   redirectURL.Port(),
			Path:   redirectURL.Path,
		}
	}

	return cReq, cRes, nil
}

var startLineRegex = regexp.MustCompile(`(?m)^`)

func formatDump(data []byte, prefix string) string {
	data = startLineRegex.ReplaceAllLiteral(data, []byte(prefix))
	return string(data)
}
