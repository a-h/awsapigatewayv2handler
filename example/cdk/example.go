package main

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	awsapigatewayv2 "github.com/aws/aws-cdk-go/awscdkapigatewayv2alpha/v2"
	awsapigatewayv2integrations "github.com/aws/aws-cdk-go/awscdkapigatewayv2integrationsalpha/v2"
	awslambdago "github.com/aws/aws-cdk-go/awscdklambdagoalpha/v2"
	"github.com/aws/constructs-go/constructs/v10"
	jsii "github.com/aws/jsii-runtime-go"
)

func NewExampleStack(scope constructs.Construct, id string, props *awscdk.StackProps) awscdk.Stack {
	stack := awscdk.NewStack(scope, &id, props)

	bundlingOptions := &awslambdago.BundlingOptions{
		GoBuildFlags: &[]*string{jsii.String(`-ldflags "-s -w" -tags lambda.norpc`)},
	}
	f := awslambdago.NewGoFunction(stack, jsii.String("handler"), &awslambdago.GoFunctionProps{
		Runtime:      awslambda.Runtime_PROVIDED_AL2(),
		Architecture: awslambda.Architecture_ARM_64(),
		Entry:        jsii.String("../lambda"),
		Bundling:     bundlingOptions,
		MemorySize:   jsii.Number(1024),
		Timeout:      awscdk.Duration_Millis(jsii.Number(15000)),
		Tracing:      awslambda.Tracing_ACTIVE,
	})
	otelLambdaLayerARN := fmt.Sprintf("arn:aws:lambda:%s:901920570463:layer:aws-otel-collector-arm64-ver-0-51-0:1", *stack.Region())
	otelLambdaLayer := awslambda.LayerVersion_FromLayerVersionArn(stack, jsii.String("otelLambdaLayer"), jsii.String(otelLambdaLayerARN))
	f.AddLayers(otelLambdaLayer)
	// Add a Function URL.
	url := f.AddFunctionUrl(&awslambda.FunctionUrlOptions{
		AuthType: awslambda.FunctionUrlAuthType_NONE,
	})
	awscdk.NewCfnOutput(stack, jsii.String("lambdaFunctionUrl"), &awscdk.CfnOutputProps{
		ExportName: jsii.String("lambdaFunctionUrl"),
		Value:      url.Url(),
	})

	// Use an API Gateway V2 endpoint.
	fi := awsapigatewayv2integrations.NewHttpLambdaIntegration(jsii.String("handlerIntegration"), f, &awsapigatewayv2integrations.HttpLambdaIntegrationProps{})
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
	NewExampleStack(app, "ExampleStack", &awscdk.StackProps{})
	app.Synth(nil)
}
