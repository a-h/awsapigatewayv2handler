# awsapigatewayv2handler

Coverts a Go `http.Handler` to a Lambda handler for API Gateway V2 requests.

```go
import "github.com/a-h/awsapigatewayv2handler"
```

## Example

### Lambda

```go
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
)

//go:embed static
var static embed.FS

type Data struct {
	Now time.Time `json:"now"`
}

func main() {
	http.Handle("/hello", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "Hello")
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

	// This handler can work as a Lambda, or a local web server.
	if os.Getenv("RUN_WEBSERVER") != "" {
		fmt.Println("Listening on port 8000")
		http.ListenAndServe("localhost:8000", http.DefaultServeMux)
		return
	}

	// Start the Lambda handler.
	awsapigatewayv2handler.ListenAndServe(http.DefaultServeMux, nil)
}

```

### CDK

```go
package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	awsapigatewayv2 "github.com/aws/aws-cdk-go/awscdkapigatewayv2alpha/v2"
	awsapigatewayv2integrations "github.com/aws/aws-cdk-go/awscdkapigatewayv2integrationsalpha/v2"
	awslambdago "github.com/aws/aws-cdk-go/awscdklambdagoalpha/v2"
	"github.com/aws/constructs-go/constructs/v10"
	jsii "github.com/aws/jsii-runtime-go"
)

type ExampleStackProps struct {
	awscdk.StackProps
}

func NewExampleStack(scope constructs.Construct, id string, props *ExampleStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	bundlingOptions := &awslambdago.BundlingOptions{
		GoBuildFlags: &[]*string{jsii.String(`-ldflags "-s -w"`)},
	}

	f := awslambdago.NewGoFunction(stack, jsii.String("handler"), &awslambdago.GoFunctionProps{
		Runtime:    awslambda.Runtime_GO_1_X(),
		Entry:      jsii.String("../lambda"),
		Bundling:   bundlingOptions,
		MemorySize: jsii.Number(1024),
		Timeout:    awscdk.Duration_Millis(jsii.Number(15000)),
	})
	fi := awsapigatewayv2integrations.NewHttpLambdaIntegration(jsii.String("handlerIntegration"), f, &awsapigatewayv2integrations.HttpLambdaIntegrationProps{
	})
	endpoint := awsapigatewayv2.NewHttpApi(stack, jsii.String("apigatewayV2Example"), &awsapigatewayv2.HttpApiProps{
		DefaultIntegration: fi,
	})
	awscdk.NewCfnOutput(stack, jsii.String("apigatewayV2ExampleUrl"), &awscdk.CfnOutputProps{
		ExportName: jsii.String("apigatewayV2ExampleUrl"),
		Value:      endpoint.Url(),
	})

	return stack
}

func main() {
	app := awscdk.NewApp(nil)
	NewExampleStack(app, "ExampleStack", &ExampleStackProps{
		awscdk.StackProps{},
	})
	app.Synth(nil)
}
```
