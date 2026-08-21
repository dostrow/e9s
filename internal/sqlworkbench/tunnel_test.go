package sqlworkbench

import (
	"reflect"
	"testing"
)

func TestSSMTunnelArguments(t *testing.T) {
	want := []string{"ssm", "start-session", "--target", "i-123", "--document-name", "AWS-StartPortForwardingSessionToRemoteHost",
		"--parameters", "host=db.example,portNumber=5432,localPortNumber=15432", "--profile", "prod", "--region", "us-east-2"}
	got := SSMTunnelArguments("i-123", "db.example", 5432, 15432, "prod", "us-east-2")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("arguments = %#v", got)
	}
}
