package aws

import "testing"

func TestLambdaARNFromIntegrationURI(t *testing.T) {
	uri := "arn:aws:apigateway:us-east-2:lambda:path/2015-03-31/functions/arn:aws:lambda:us-east-2:123456789012:function:orders/invocations"
	want := "arn:aws:lambda:us-east-2:123456789012:function:orders"
	if got := lambdaARNFromIntegrationURI(uri); got != want {
		t.Fatalf("lambdaARNFromIntegrationURI() = %q, want %q", got, want)
	}
}
