package model

import (
	"strings"
	"testing"
)

func TestExecSessionPluginArguments(t *testing.T) {
	session := &ExecSession{
		SessionID: "session-1", StreamURL: "wss://example", TokenValue: "token",
		Region: "us-east-1", Target: "ecs:cluster_task_container",
	}
	args, err := session.BuildPluginArgs()
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 6 || args[1] != "us-east-1" || args[2] != "StartSession" || !strings.Contains(args[4], session.Target) || args[5] != "https://ecs.us-east-1.amazonaws.com" {
		t.Fatalf("BuildPluginArgs() = %#v", args)
	}
	ssmArgs, err := session.BuildSSMPluginArgs()
	if err != nil {
		t.Fatal(err)
	}
	if ssmArgs[5] != "https://ssm.us-east-1.amazonaws.com" {
		t.Fatalf("BuildSSMPluginArgs() = %#v", ssmArgs)
	}
}
