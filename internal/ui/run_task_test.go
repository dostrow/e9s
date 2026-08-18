package ui

import (
	"reflect"
	"testing"
)

func TestRunTaskFormBuildsRequest(t *testing.T) {
	form := NewRunTaskForm("cluster-a", "api:42")
	form.launchType = 1
	form.count.SetValue("2")
	form.subnets.SetValue("subnet-a, subnet-b")
	form.securityGroups.SetValue("sg-a")
	form.group.SetValue("batch")
	form.assignPublicIP = true
	form.enableExec = true

	request, err := form.request()
	if err != nil {
		t.Fatal(err)
	}
	if request.Cluster != "cluster-a" || request.TaskDefinition != "api:42" || request.LaunchType != "FARGATE" || request.Count != 2 {
		t.Fatalf("request core fields = %#v", request)
	}
	if !reflect.DeepEqual(request.Subnets, []string{"subnet-a", "subnet-b"}) || !reflect.DeepEqual(request.SecurityGroups, []string{"sg-a"}) {
		t.Fatalf("request networking = %#v", request)
	}
	if !request.AssignPublicIP || !request.EnableExecuteCommand || request.Group != "batch" {
		t.Fatalf("request options = %#v", request)
	}
}

func TestRunTaskFormRejectsInvalidCount(t *testing.T) {
	form := NewRunTaskForm("cluster-a", "api:42")
	form.count.SetValue("0")
	if _, err := form.request(); err == nil {
		t.Fatal("request accepted a zero count")
	}
}
