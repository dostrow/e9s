package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

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

type ExecSession struct {
	SessionID  string `json:"SessionId"`
	StreamURL  string `json:"StreamUrl"`
	TokenValue string `json:"TokenValue"`
	Region     string
	Target     string
}

// SessionManagerPluginPath returns the path to session-manager-plugin if installed.
func SessionManagerPluginPath() (string, error) {
	path, err := exec.LookPath("session-manager-plugin")
	if err != nil {
		return "", fmt.Errorf("session-manager-plugin not found in PATH — install it from https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html")
	}
	return path, nil
}

// BuildPluginArgs returns the arguments for session-manager-plugin.
func (s *ExecSession) BuildPluginArgs() ([]string, error) {
	sessionJSON, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return []string{
		string(sessionJSON),
		s.Region,
		"StartSession",
		"",
		fmt.Sprintf(`{"Target":"%s"}`, s.Target),
		fmt.Sprintf("https://ecs.%s.amazonaws.com", s.Region),
	}, nil
}

func derefStrAws(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}
