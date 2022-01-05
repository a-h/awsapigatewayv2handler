package awsapigatewayv2handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
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
		Handler: h,
	}
}

type LambdaHandler struct {
	Handler http.Handler
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

	// Execute the request.
	w := httptest.NewRecorder()
	lh.Handler.ServeHTTP(w, r.WithContext(ctx))

	// Convert the recorded result to an API Gateway response.
	return lh.convertHTTPResponseToLambdaEvent(w)
}

func (lh LambdaHandler) convertLambdaEventToHTTPRequest(e events.APIGatewayV2HTTPRequest) (req *http.Request, err error) {
	body, cl := getRequestBody(e.Body, e.IsBase64Encoded)
	req, err = http.NewRequest(e.RequestContext.HTTP.Method, e.RawPath, body)
	req.URL.RawQuery = e.RawQueryString
	for k, v := range e.Headers {
		req.Header.Add(k, v)
	}
	if cl > 0 {
		req.Header.Set("Content-Length", strconv.Itoa(cl))
		req.ContentLength = int64(cl)
	}
	return
}

func getRequestBody(s string, isBase64Encoded bool) (body io.Reader, contentLength int) {
	if s == "" {
		return nil, -1
	}
	if isBase64Encoded {
		var padding int
		if len(s) > 1 {
			for _, c := range s[len(s)-2:] {
				if c == '=' {
					padding++
				}
			}
		}
		contentLength = (3 * (len(s) / 4)) - padding
		return base64.NewDecoder(base64.StdEncoding, bytes.NewReader([]byte(s))), contentLength
	}
	return bytes.NewReader([]byte(s)), len(s)
}

func (lh LambdaHandler) convertHTTPResponseToLambdaEvent(rec *httptest.ResponseRecorder) (resp events.APIGatewayV2HTTPResponse, err error) {
	result := rec.Result()
	resp.StatusCode = result.StatusCode
	resp.Body, resp.IsBase64Encoded = lh.getResponseBody(rec)
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

func (lh LambdaHandler) getResponseBody(rec *httptest.ResponseRecorder) (body string, isBase64Encoded bool) {
	if isTextType(rec.HeaderMap.Get("Content-Type")) {
		return rec.Body.String(), false
	}
	return base64.StdEncoding.EncodeToString(rec.Body.Bytes()), true
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
