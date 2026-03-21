package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	awsPkg "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	smSvc "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	ssmSvc "github.com/aws/aws-sdk-go-v2/service/ssm"
)

type TaskDefSummary struct {
	Family    string
	Revision  int
	CPU       string
	Memory    string
	Containers []TaskDefContainer
	RawJSON   string
}

type TaskDefContainer struct {
	Name       string
	Image      string
	CPU        int
	Memory     int
	Essential  bool
	EnvVars    []EnvVar
	EnvVarKeys []string // kept for diff compatibility
}

type EnvVar struct {
	Name          string
	Value         string // plain value for env, ARN for secrets
	ResolvedValue string // resolved secret value (populated by ResolveEnvVars)
	Source        string // "" for plain env, "secrets-manager" or "ssm" for secrets
}

// GetTaskDefinition fetches and summarizes a task definition.
func (c *Client) GetTaskDefinition(ctx context.Context, taskDef string) (*TaskDefSummary, error) {
	out, err := c.ECS.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: &taskDef,
	})
	if err != nil {
		return nil, err
	}

	td := out.TaskDefinition
	summary := &TaskDefSummary{
		Family:   derefStrAws(td.Family),
		Revision: int(td.Revision),
		CPU:      derefStrAws(td.Cpu),
		Memory:   derefStrAws(td.Memory),
	}

	for _, cd := range td.ContainerDefinitions {
		mem := 0
		if cd.Memory != nil {
			mem = int(*cd.Memory)
		}
		c := TaskDefContainer{
			Name:      derefStrAws(cd.Name),
			Image:     derefStrAws(cd.Image),
			CPU:       int(cd.Cpu),
			Memory:    mem,
			Essential: cd.Essential != nil && *cd.Essential,
		}
		for _, env := range cd.Environment {
			if env.Name != nil {
				c.EnvVarKeys = append(c.EnvVarKeys, *env.Name)
				val := ""
				if env.Value != nil {
					val = *env.Value
				}
				c.EnvVars = append(c.EnvVars, EnvVar{Name: *env.Name, Value: val})
			}
		}
		// Secrets injected as env vars (from SSM Parameter Store / Secrets Manager)
		for _, sec := range cd.Secrets {
			if sec.Name != nil {
				c.EnvVarKeys = append(c.EnvVarKeys, *sec.Name)
				valueFrom := ""
				source := ""
				if sec.ValueFrom != nil {
					valueFrom = *sec.ValueFrom
					if strings.Contains(valueFrom, "secretsmanager") {
						source = "secrets-manager"
					} else if strings.Contains(valueFrom, "ssm") || strings.Contains(valueFrom, "parameter") {
						source = "ssm"
					} else {
						source = "secret"
					}
				}
				c.EnvVars = append(c.EnvVars, EnvVar{
					Name:   *sec.Name,
					Value:  valueFrom,
					Source: source,
				})
			}
		}
		summary.Containers = append(summary.Containers, c)
	}

	raw, _ := json.MarshalIndent(td, "", "  ")
	summary.RawJSON = string(raw)

	return summary, nil
}

// DiffTaskDefinitions produces a human-readable diff between two task definitions.
func DiffTaskDefinitions(old, new *TaskDefSummary) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("--- %s:%d\n", old.Family, old.Revision))
	b.WriteString(fmt.Sprintf("+++ %s:%d\n", new.Family, new.Revision))
	b.WriteString("\n")

	if old.CPU != new.CPU {
		b.WriteString(fmt.Sprintf("  CPU: %s → %s\n", old.CPU, new.CPU))
	}
	if old.Memory != new.Memory {
		b.WriteString(fmt.Sprintf("  Memory: %s → %s\n", old.Memory, new.Memory))
	}

	// Index old containers by name
	oldMap := map[string]TaskDefContainer{}
	for _, c := range old.Containers {
		oldMap[c.Name] = c
	}

	for _, nc := range new.Containers {
		oc, exists := oldMap[nc.Name]
		if !exists {
			b.WriteString(fmt.Sprintf("\n  + Container added: %s\n", nc.Name))
			b.WriteString(fmt.Sprintf("    Image: %s\n", nc.Image))
			continue
		}
		delete(oldMap, nc.Name)

		header := false
		writeHeader := func() {
			if !header {
				b.WriteString(fmt.Sprintf("\n  Container: %s\n", nc.Name))
				header = true
			}
		}

		if oc.Image != nc.Image {
			writeHeader()
			b.WriteString(fmt.Sprintf("    Image: %s → %s\n", oc.Image, nc.Image))
		}
		if oc.CPU != nc.CPU {
			writeHeader()
			b.WriteString(fmt.Sprintf("    CPU: %d → %d\n", oc.CPU, nc.CPU))
		}
		if oc.Memory != nc.Memory {
			writeHeader()
			b.WriteString(fmt.Sprintf("    Memory: %d → %d\n", oc.Memory, nc.Memory))
		}

		// Diff env var keys
		oldEnv := toSet(oc.EnvVarKeys)
		newEnv := toSet(nc.EnvVarKeys)
		for k := range newEnv {
			if !oldEnv[k] {
				writeHeader()
				b.WriteString(fmt.Sprintf("    + Env: %s\n", k))
			}
		}
		for k := range oldEnv {
			if !newEnv[k] {
				writeHeader()
				b.WriteString(fmt.Sprintf("    - Env: %s\n", k))
			}
		}
	}

	for name := range oldMap {
		b.WriteString(fmt.Sprintf("\n  - Container removed: %s\n", name))
	}

	result := b.String()
	if result == fmt.Sprintf("--- %s:%d\n+++ %s:%d\n\n", old.Family, old.Revision, new.Family, new.Revision) {
		return "No differences found."
	}
	return result
}

// ResolveEnvVars resolves secret references to their actual values.
// SSM parameters are fetched with decryption, Secrets Manager secrets
// are fetched and JSON keys extracted if specified in the ARN.
func (c *Client) ResolveEnvVars(ctx context.Context, envVars []EnvVar) []EnvVar {
	resolved := make([]EnvVar, len(envVars))
	copy(resolved, envVars)

	// Batch SSM parameter names
	var ssmNames []string
	ssmIndex := map[string][]int{} // param name -> indices in resolved

	for i, ev := range resolved {
		if ev.Source == "" {
			resolved[i].ResolvedValue = ev.Value
			continue
		}
		if ev.Source == "ssm" {
			// SSM ARN format: arn:aws:ssm:region:account:parameter/name or just /name
			paramName := extractSSMParamName(ev.Value)
			ssmNames = append(ssmNames, paramName)
			ssmIndex[paramName] = append(ssmIndex[paramName], i)
		}
	}

	// Batch fetch SSM parameters (max 10 per call)
	if len(ssmNames) > 0 {
		c.resolveSSMBatch(ctx, ssmNames, ssmIndex, resolved)
	}

	// Resolve Secrets Manager secrets one at a time (they may have JSON key selectors)
	for i, ev := range resolved {
		if ev.Source == "secrets-manager" {
			val, err := c.resolveSecretValue(ctx, ev.Value)
			if err != nil {
				resolved[i].ResolvedValue = "(error: " + err.Error() + ")"
			} else {
				resolved[i].ResolvedValue = val
			}
		}
	}

	return resolved
}

func (c *Client) resolveSSMBatch(ctx context.Context, names []string, index map[string][]int, resolved []EnvVar) {
	for i := 0; i < len(names); i += 10 {
		end := i + 10
		if end > len(names) {
			end = len(names)
		}
		batch := names[i:end]

		out, err := c.SSM.GetParameters(ctx, &ssmSvc.GetParametersInput{
			Names:          batch,
			WithDecryption: awsPkg.Bool(true),
		})
		if err != nil {
			for _, name := range batch {
				for _, idx := range index[name] {
					resolved[idx].ResolvedValue = "(error resolving)"
				}
			}
			continue
		}
		for _, p := range out.Parameters {
			if p.Name != nil && p.Value != nil {
				for _, idx := range index[*p.Name] {
					resolved[idx].ResolvedValue = *p.Value
				}
			}
		}
		for _, inv := range out.InvalidParameters {
			for _, idx := range index[inv] {
				resolved[idx].ResolvedValue = "(not found)"
			}
		}
	}
}

func (c *Client) resolveSecretValue(ctx context.Context, valueFrom string) (string, error) {
	// valueFrom format: arn:aws:secretsmanager:region:account:secret:name:jsonkey:version:stage
	// or: arn:aws:secretsmanager:region:account:secret:name
	// The JSON key, version, and stage are optional suffixes separated by ':'

	secretID, jsonKey := parseSecretARN(valueFrom)

	out, err := c.SM.GetSecretValue(ctx, &smSvc.GetSecretValueInput{
		SecretId: &secretID,
	})
	if err != nil {
		return "", err
	}
	if out.SecretString == nil {
		return "(binary secret)", nil
	}

	secretStr := *out.SecretString

	if jsonKey != "" {
		// Try to extract the JSON key
		val, err := extractJSONKey(secretStr, jsonKey)
		if err != nil {
			return secretStr, nil // fallback to full string
		}
		return val, nil
	}

	return secretStr, nil
}

// parseSecretARN splits a Secrets Manager valueFrom into the secret ARN and optional JSON key.
// Format: arn:aws:secretsmanager:region:account:secret:name:jsonkey::
func parseSecretARN(valueFrom string) (string, string) {
	// Count colons to find the structure
	// Full ARN has 6 colons for arn:aws:secretsmanager:region:account:secret:name
	// With JSON key: arn:aws:secretsmanager:region:account:secret:name:jsonkey::
	parts := strings.Split(valueFrom, ":")

	if len(parts) <= 7 {
		// No JSON key suffix
		return valueFrom, ""
	}

	// Reconstruct the base ARN (first 7 parts)
	secretARN := strings.Join(parts[:7], ":")
	jsonKey := parts[7]
	return secretARN, jsonKey
}

func extractJSONKey(jsonStr, key string) (string, error) {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		return "", err
	}
	val, ok := m[key]
	if !ok {
		return "", fmt.Errorf("key %q not found", key)
	}
	return fmt.Sprintf("%v", val), nil
}

// extractSSMParamName extracts the parameter name from an SSM ARN or returns as-is.
// ARN format: arn:aws:ssm:region:account:parameter/path/to/param
func extractSSMParamName(arnOrName string) string {
	if strings.HasPrefix(arnOrName, "arn:") {
		if idx := strings.Index(arnOrName, ":parameter"); idx != -1 {
			return arnOrName[idx+len(":parameter"):]
		}
	}
	return arnOrName
}

func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}
