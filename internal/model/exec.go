package model

import (
	"encoding/json"
	"fmt"
)

type ExecSession struct {
	SessionID  string `json:"SessionId"`
	StreamURL  string `json:"StreamUrl"`
	TokenValue string `json:"TokenValue"`
	Region     string
	Target     string
}

type ExecLaunch struct {
	Executable string
	Args       []string
}

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

func (s *ExecSession) BuildSSMPluginArgs() ([]string, error) {
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
		fmt.Sprintf("https://ssm.%s.amazonaws.com", s.Region),
	}, nil
}
