module example

go 1.16

replace github.com/a-h/awsapigatewayv2handler => ../

require (
	github.com/a-h/awsapigatewayv2handler v0.0.0-20220713111419-eae1b0de1c53
	github.com/aws/aws-cdk-go/awscdk/v2 v2.2.0
	github.com/aws/aws-cdk-go/awscdkapigatewayv2alpha/v2 v2.2.0-alpha.0
	github.com/aws/aws-cdk-go/awscdkapigatewayv2integrationsalpha/v2 v2.2.0-alpha.0
	github.com/aws/aws-cdk-go/awscdklambdagoalpha/v2 v2.2.0-alpha.0
	github.com/aws/aws-lambda-go v1.27.1 // indirect
	github.com/aws/aws-xray-sdk-go v1.6.1-0.20211110224843-1f272e4024a5
	github.com/aws/constructs-go/constructs/v10 v10.0.9
	github.com/aws/jsii-runtime-go v1.49.0
	github.com/joe-davidson1802/zapray v0.0.17-0.20211218172612-ff5bd3fd0d43
	go.uber.org/zap v1.19.1
	golang.org/x/net v0.0.0-20210510120150-4163338589ed
)
