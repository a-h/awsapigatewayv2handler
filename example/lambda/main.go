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

	"github.com/a-h/awsapigatewayv2handler"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/joe-davidson1802/zapray"
	"go.uber.org/zap"
	"golang.org/x/net/context/ctxhttp"
)

//go:embed static
var static embed.FS

type Data struct {
	Now time.Time `json:"now"`
}

// X-Ray compatible logger.
var logger *zapray.Logger

func init() {
	var err error
	logger, err = zapray.NewProduction()
	if err != nil {
		panic("unable to create logger: " + err.Error())
	}
}

func main() {
	http.Handle("/hello", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "Hello")
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
		log := logger.TraceRequest(r)
		log.Info("making client")
		client := xray.Client(nil)
		resp, err := ctxhttp.Get(r.Context(), client, "https://jsonplaceholder.typicode.com/posts")
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

	// Start the Lambda handler.
	// Use X-Ray middleware to track everything.
	withXRay := XRayMiddleware{
		Name: "awsapigatewayv2handlerExample",
		Next: http.DefaultServeMux,
	}
	awsapigatewayv2handler.ListenAndServe(withXRay)
}

type XRayMiddleware struct {
	Name string
	Next http.Handler
}

func (m XRayMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// It should be possible to connect the subsegment from the Lambda context, but XRay seems incapable of doing so.
	ctx, s := xray.BeginSegment(r.Context(), m.Name)
	defer s.Close(nil)
	h := xray.HandlerWithContext(ctx, xray.NewFixedSegmentNamer(m.Name), m.Next)
	h.ServeHTTP(w, r)
}
