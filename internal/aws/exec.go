package aws

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/dostrow/e9s/internal/model"
)

type ExecSession = model.ExecSession

// ExecuteCommand initiates an ECS Exec session and returns the session info
// needed for session-manager-plugin.
func (c *Client) ExecuteCommand(ctx context.Context, cluster, taskARN, container, command string) (*ExecSession, error) {
	out, err := c.ECS.ExecuteCommand(ctx, &ecs.ExecuteCommandInput{
		Cluster:     &cluster,
		Task:        &taskARN,
		Container:   &container,
		Command:     &command,
		Interactive: true,
	})
	if err != nil {
		return nil, fmt.Errorf("execute-command failed: %w", err)
	}

	if out.Session == nil {
		return nil, fmt.Errorf("no session returned from execute-command")
	}

	return &ExecSession{
		SessionID:  derefStrAws(out.Session.SessionId),
		StreamURL:  derefStrAws(out.Session.StreamUrl),
		TokenValue: derefStrAws(out.Session.TokenValue),
		Region:     c.Region(),
		Target:     fmt.Sprintf("ecs:%s_%s_%s", cluster, taskARN, container),
	}, nil
}

// SessionManagerPluginPath returns the path to session-manager-plugin if installed.
func SessionManagerPluginPath() (string, error) {
	path, err := exec.LookPath("session-manager-plugin")
	if err != nil {
		return "", fmt.Errorf("session-manager-plugin not found in PATH — install it from https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html")
	}
	return path, nil
}

func derefStrAws(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}
