package awsapigatewayv2handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func ListenAndServe(h http.Handler) {
	if h == nil {
		h = http.DefaultServeMux
	}
	lambda.StartHandler(NewLambdaHandler(h))
}

func NewLambdaHandler(h http.Handler) LambdaHandler {
	return LambdaHandler{
		Handler:                h,
		HandlerResponseHeaders: make(http.Header),
		HandlerResponseBuffer:  new(bytes.Buffer),
		base64Buffer:           new(bytes.Buffer),
	}
}

type LambdaHandler struct {
	Handler                http.Handler
	HandlerResponseHeaders http.Header
	HandlerResponseBuffer  *bytes.Buffer
	base64Buffer           *bytes.Buffer
}

func (lh LambdaHandler) Invoke(ctx context.Context, payload []byte) ([]byte, error) {
	var req events.APIGatewayV2HTTPRequest
	err := json.Unmarshal(payload, &req)
	if err != nil {
		return nil, err
	}
	resp, err := lh.Handle(ctx, req)
	if err != nil {
		return nil, err
	}
	return json.Marshal(resp)
}

func (lh LambdaHandler) Handle(ctx context.Context, e events.APIGatewayV2HTTPRequest) (resp events.APIGatewayV2HTTPResponse, err error) {
	// Convert the event to a HTTP request.
	r, err := lh.convertLambdaEventToHTTPRequest(e)
	if err != nil {
		return
	}

	// Clear the reusable header map and buffer.
	for k := range lh.HandlerResponseHeaders {
		delete(lh.HandlerResponseHeaders, k)
	}
	lh.HandlerResponseBuffer.Reset()
	// Execute the request.
	w := &httptest.ResponseRecorder{
		HeaderMap: lh.HandlerResponseHeaders,
		Body:      lh.HandlerResponseBuffer,
		Code:      200,
	}
	lh.Handler.ServeHTTP(w, r)

	// Convert the recorded result to an API Gateway response.
	return lh.convertHTTPResponseToLambdaEvent(w.Result())
}

func (lh LambdaHandler) convertLambdaEventToHTTPRequest(e events.APIGatewayV2HTTPRequest) (req *http.Request, err error) {
	u := e.RawPath
	var body io.Reader
	if e.Body != "" {
		if e.IsBase64Encoded {
			data, err := base64.StdEncoding.DecodeString(e.Body)
			if err != nil {
				err = fmt.Errorf("awsapigatewayv2handler: failed to get event body: %w", err)
				return req, err
			}
			body = bytes.NewReader(data)
		} else {
			body = bytes.NewReader([]byte(e.Body))
		}
	}
	req, err = http.NewRequest(e.RequestContext.HTTP.Method, u, body)
	req.URL.RawQuery = e.RawQueryString
	for k, v := range e.Headers {
		req.Header.Add(k, v)
	}
	return
}

func (lh LambdaHandler) convertHTTPResponseToLambdaEvent(result *http.Response) (resp events.APIGatewayV2HTTPResponse, err error) {
	resp.StatusCode = result.StatusCode
	if isTextType(result.Header.Get("Content-Type")) {
		resp.IsBase64Encoded = false
		resp.Body = lh.HandlerResponseBuffer.String()
	} else {
		resp.IsBase64Encoded = true
		lh.base64Buffer.Reset()
		enc := base64.NewEncoder(base64.StdEncoding, lh.base64Buffer)
		_, err = enc.Write(lh.HandlerResponseBuffer.Bytes())
		if err != nil {
			return
		}
		err = enc.Close()
		if err != nil {
			return
		}
		resp.Body = lh.base64Buffer.String()
	}
	resp.MultiValueHeaders = result.Header
	if result.ContentLength > -1 {
		resp.MultiValueHeaders["Content-Length"] = []string{strconv.FormatInt(result.ContentLength, 10)}
	}
	for k, v := range result.Trailer {
		resp.MultiValueHeaders[k] = v
	}
	cookies := result.Cookies()
	if len(cookies) > 0 {
		resp.Cookies = make([]string, len(cookies))
		for i := 0; i < len(cookies); i++ {
			resp.Cookies[i] = cookies[i].String()
		}
	}
	return
}

func isTextType(contentType string) bool {
	if contentType == "" {
		// API Gateway's default Content-Type is application/json
		// See https://docs.aws.amazon.com/apigateway/latest/developerguide/request-response-data-mappings.html
		return true
	}
	if strings.HasPrefix(contentType, "text/") {
		return true
	}
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Basics_of_HTTP/MIME_types/Common_types
	if contentType == "application/json" {
		return true
	}
	if contentType == "image/svg+xml" {
		return true
	}
	if contentType == "application/xhtml+xml" {
		return true
	}
	if contentType == "application/xml" {
		return true
	}
	return false
}
