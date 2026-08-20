package sqlworkbench

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dostrow/e9s/internal/config"
)

type TunnelOptions struct {
	AWSProfile string
	AWSRegion  string
	Timeout    time.Duration
}

type Tunnel struct {
	LocalAddress string
	cancel       context.CancelFunc
	done         chan error
	once         sync.Once
}

func StartSSMTunnel(ctx context.Context, settings config.SSMTunnel, remoteHost string, remotePort int, options TunnelOptions) (*Tunnel, error) {
	if strings.TrimSpace(settings.InstanceID) == "" {
		return nil, fmt.Errorf("SSM tunnel instance ID is required")
	}
	if strings.TrimSpace(settings.RemoteHost) != "" {
		remoteHost = strings.TrimSpace(settings.RemoteHost)
	}
	if settings.RemotePort > 0 {
		remotePort = settings.RemotePort
	}
	if remoteHost == "" || remotePort <= 0 {
		return nil, fmt.Errorf("SSM tunnel requires a remote host and port")
	}
	localPort := settings.LocalPort
	if localPort == 0 {
		var err error
		localPort, err = freeLocalPort()
		if err != nil {
			return nil, err
		}
	}
	arguments := SSMTunnelArguments(settings.InstanceID, remoteHost, remotePort, localPort, options.AWSProfile, options.AWSRegion)
	processContext, cancel := context.WithCancel(context.Background())
	command := exec.CommandContext(processContext, "aws", arguments...)
	var diagnostics bytes.Buffer
	command.Stdout = &diagnostics
	command.Stderr = &diagnostics
	if err := command.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start AWS SSM tunnel: %w", err)
	}
	tunnel := &Tunnel{LocalAddress: net.JoinHostPort("127.0.0.1", strconv.Itoa(localPort)), cancel: cancel, done: make(chan error, 1)}
	go func() { tunnel.done <- command.Wait() }()
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			tunnel.Close()
			return nil, ctx.Err()
		case err := <-tunnel.done:
			cancel()
			message := strings.TrimSpace(diagnostics.String())
			if len(message) > 500 {
				message = message[len(message)-500:]
			}
			if message != "" {
				return nil, fmt.Errorf("AWS SSM tunnel exited before becoming ready: %v: %s", err, message)
			}
			return nil, fmt.Errorf("AWS SSM tunnel exited before becoming ready: %v", err)
		case <-deadline.C:
			tunnel.Close()
			return nil, fmt.Errorf("timed out waiting for AWS SSM tunnel on %s", tunnel.LocalAddress)
		case <-ticker.C:
			connection, err := net.DialTimeout("tcp", tunnel.LocalAddress, 100*time.Millisecond)
			if err == nil {
				connection.Close()
				return tunnel, nil
			}
		}
	}
}

func SSMTunnelArguments(instanceID, remoteHost string, remotePort, localPort int, profile, region string) []string {
	arguments := []string{"ssm", "start-session", "--target", instanceID, "--document-name", "AWS-StartPortForwardingSessionToRemoteHost",
		"--parameters", fmt.Sprintf("host=%s,portNumber=%d,localPortNumber=%d", remoteHost, remotePort, localPort)}
	if strings.TrimSpace(profile) != "" {
		arguments = append(arguments, "--profile", strings.TrimSpace(profile))
	}
	if strings.TrimSpace(region) != "" {
		arguments = append(arguments, "--region", strings.TrimSpace(region))
	}
	return arguments
}

func (t *Tunnel) Close() {
	if t == nil {
		return
	}
	t.once.Do(t.cancel)
}

func freeLocalPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}
