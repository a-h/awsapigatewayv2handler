package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	_ "embed"

	"context"

	"github.com/a-h/awsapigatewayv2handler"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-lambda-go/otellambda"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-lambda-go/otellambda/xrayconfig"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/contrib/propagators/aws/xray"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

//go:embed static
var static embed.FS

type Data struct {
	Now time.Time `json:"now"`
}

var logger *zap.Logger

func init() {
	var err error
	logger, err = zap.NewProduction()
	if err != nil {
		panic("unable to create logger: " + err.Error())
	}
}

func main() {
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		logger.Fatal("failed to load AWS config", zap.Error(err))
	}

	// Configure Lambda functions.
	http.Handle("/hello", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "Hello")
	}))
	http.Handle("/dynamofail", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := withTraceIDLogger(r.Context(), logger)
		svc := dynamodb.NewFromConfig(cfg)
		_, err := svc.GetItem(r.Context(), &dynamodb.GetItemInput{
			TableName: aws.String("apigatewayv2example-table"), // Doesn't exist. Expect this to fail.
			Key: map[string]types.AttributeValue{
				"_pk": &types.AttributeValueMemberS{Value: "123"},
			},
		})
		log.Error("dynamodb error", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}))
	http.Handle("/smile", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("content-type", "image/svg+xml")
		// SVG from Heroicons, MIT licensed: https://heroicons.com/
		io.WriteString(w, `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
  <path stroke-linecap="round" stroke-linejoin="round" d="M14.828 14.828a4 4 0 01-5.656 0M9 10h.01M15 10h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
</svg>`)
	}))
	http.Handle("/json", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := Data{Now: time.Now()}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(data); err != nil {
			http.Error(w, "failed to encode JSON", http.StatusInternalServerError)
			return
		}
	}))
	// Serve the static directory that has been embedded into the binary.
	// The static directory contains a mix of binary and text files, for testing.
	http.Handle("/static/", http.FileServer(http.FS(static)))
	http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "Index")
	}))
	http.Handle("/xray", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log with the @xrayTraceId field to include log entries show up alongside X-Ray traces in the AWS console.
		log := withTraceIDLogger(r.Context(), logger)
		// Use an instrumented HTTP client if available (it falls back to the default HTTP client).
		client := otelhttp.DefaultClient
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, "https://jsonplaceholder.typicode.com/posts", nil)
		if err != nil {
			log.Error("failed to create request", zap.Error(err))
			http.Error(w, "failed to create request", http.StatusInternalServerError)
			return
		}
		resp, err := client.Do(req.WithContext(r.Context()))
		if err != nil {
			log.Error("failed to make request", zap.Error(err))
			http.Error(w, "failed to make request", http.StatusInternalServerError)
			return
		}
		log.Info("returning repsonse")
		io.Copy(w, resp.Body)
	}))

	// This handler can work as a Lambda, or a local web server.
	if os.Getenv("RUN_WEBSERVER") != "" {
		fmt.Println("Listening on port 8000")
		http.ListenAndServe("localhost:8000", http.DefaultServeMux)
		return
	}

	// Set up telemetry.
	tp, err := xrayconfig.NewTracerProvider(ctx)
	if err != nil {
		logger.Fatal("failed to create tracer provider", zap.Error(err))
	}
	defer func(ctx context.Context) {
		if err := tp.Shutdown(ctx); err != nil {
			logger.Error("failed to shut down trace provider", zap.Error(err))
		}
	}(ctx)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(xray.Propagator{})
	// Instrument AWS SDK.
	otelaws.AppendMiddlewares(&cfg.APIOptions)

	// Start Lambda function handler.
	handler := awsapigatewayv2handler.NewLambdaHandler(http.DefaultServeMux)
	withTelemetry := otellambda.InstrumentHandler(handler.Handle, xrayconfig.WithRecommendedOptions(tp)...)
	lambda.Start(withTelemetry)

	// If you don't need X-Ray, you can use ListenAndServe directly instead of calling lambda.StartHandler or
	// lambda.Start yourself.
	// awsapigatewayv2handler.ListenAndServe(http.DefaultServeMux)
}

func withTraceIDLogger(ctx context.Context, log *zap.Logger) *zap.Logger {
	span := trace.SpanFromContext(ctx)
	traceID := span.SpanContext().TraceID().String()
	return log.With(zap.String("@xrayTraceId", fmt.Sprintf("1-%s-%s", traceID[0:8], traceID[8:])))
}
