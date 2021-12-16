package awsapigatewayv2handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type Options struct {
}

var defaultOptions Options = Options{}

func ListenAndServe(h http.Handler, opts *Options) {
	if h == nil {
		h = http.DefaultServeMux
	}
	if opts == nil {
		opts = &defaultOptions
	}
	lh := LambdaHandler{Handler: h, Options: *opts}
	lambda.StartHandler(lh)
}

type LambdaHandler struct {
	Handler http.Handler
	Options Options
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
	buf := new(bytes.Buffer)
	err = json.NewEncoder(buf).Encode(resp)
	return buf.Bytes(), err
}

func (lh LambdaHandler) Handle(ctx context.Context, e events.APIGatewayV2HTTPRequest) (resp events.APIGatewayV2HTTPResponse, err error) {
	// Convert the event to a HTTP request.
	r, err := convertLambdaEventToHTTPRequest(e)
	if err != nil {
		return
	}

	// Execute the request.
	w := httptest.NewRecorder()
	lh.Handler.ServeHTTP(w, r)

	// Convert the recorded result to an API Gateway response.
	return convertHTTPResponseToLambdaEvent(w.Result())
}

func convertLambdaEventToHTTPRequest(e events.APIGatewayV2HTTPRequest) (req *http.Request, err error) {
	u := e.RawPath
	if len(e.RawQueryString) > 0 {
		u += "?" + e.RawQueryString
	}
	body, err := getEventBody(e)
	if err != nil {
		err = fmt.Errorf("awsapigatewayv2handler: failed to get event body: %w", err)
		return
	}
	req, err = http.NewRequest(e.RequestContext.HTTP.Method, u, body)
	for k, v := range e.Headers {
		req.Header.Add(k, v)
	}
	return
}

func getEventBody(e events.APIGatewayV2HTTPRequest) (body io.Reader, err error) {
	if e.Body == "" {
		return nil, nil
	}
	if e.IsBase64Encoded {
		data, err := base64.StdEncoding.DecodeString(e.Body)
		return bytes.NewReader(data), err
	}
	return bytes.NewReader([]byte(e.Body)), nil
}

func convertHTTPResponseToLambdaEvent(result *http.Response) (resp events.APIGatewayV2HTTPResponse, err error) {
	resp.StatusCode = result.StatusCode
	resp.Body, resp.IsBase64Encoded, err = getEventBodyFromResponse(result)
	if err != nil {
		return
	}
	resp.MultiValueHeaders = result.Header.Clone()
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

func getEventBodyFromResponse(result *http.Response) (body string, isBase64Encoded bool, err error) {
	if isTextType(result.Header.Get("Content-Type")) {
		bdy, err := ioutil.ReadAll(result.Body)
		return string(bdy), false, err
	}
	op := new(bytes.Buffer)
	enc := base64.NewEncoder(base64.StdEncoding, op)
	_, err = io.Copy(enc, result.Body)
	if err != nil {
		return "", false, err
	}
	err = enc.Close()
	if err != nil {
		return "", false, err
	}
	return op.String(), true, err
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
