package sshexec

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/gsarma/localisprod-v2/internal/models"
	"golang.org/x/crypto/ssh"
)

// Runner abstracts over SSH and local execution.
type Runner interface {
	Run(cmd string) (string, error)
	Ping() error
}

// NewRunner returns a LocalRunner for local nodes, SSHClient otherwise.
func NewRunner(node *models.Node) Runner {
	if node.IsLocal {
		return &LocalRunner{}
	}
	return &Client{
		host:       node.Host,
		port:       node.Port,
		username:   node.Username,
		privateKey: node.PrivateKey,
	}
}

// LocalRunner executes commands directly on the local machine via sh.
type LocalRunner struct{}

func (l *LocalRunner) Run(cmd string) (string, error) {
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func (l *LocalRunner) Ping() error {
	_, err := l.Run("echo pong")
	return err
}

// Client executes commands on a remote host via SSH.
type Client struct {
	host       string
	port       int
	username   string
	privateKey string
}

func (c *Client) dial() (*ssh.Client, error) {
	signer, err := ssh.ParsePrivateKey([]byte(c.privateKey))
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	config := &ssh.ClientConfig{
		User: c.username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}
	addr := net.JoinHostPort(c.host, fmt.Sprintf("%d", c.port))
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	return client, nil
}

func (c *Client) Run(cmd string) (string, error) {
	client, err := c.dial()
	if err != nil {
		return "", err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	out, err := session.CombinedOutput(cmd)
	return strings.TrimSpace(string(out)), err
}

func (c *Client) Ping() error {
	_, err := c.Run("echo pong")
	return err
}

func DockerLoginCmd(username, token string) string {
	return fmt.Sprintf("echo %s | docker login ghcr.io -u %s --password-stdin",
		shellEscape(token), shellEscape(username))
}

func DockerRunCmd(containerName, image string, ports []string, envVars map[string]string, command string) string {
	var sb strings.Builder
	sb.WriteString("docker run -d --name ")
	sb.WriteString(shellEscape(containerName))

	for _, p := range ports {
		sb.WriteString(" -p ")
		sb.WriteString(shellEscape(p))
	}

	for k, v := range envVars {
		sb.WriteString(" -e ")
		sb.WriteString(shellEscape(k + "=" + v))
	}

	sb.WriteString(" ")
	sb.WriteString(shellEscape(image))

	if command != "" {
		sb.WriteString(" ")
		sb.WriteString(command)
	}

	return sb.String()
}

func DockerStopRemoveCmd(containerName string) string {
	return fmt.Sprintf("docker stop %s && docker rm %s", shellEscape(containerName), shellEscape(containerName))
}

func DockerRestartCmd(containerName string) string {
	return fmt.Sprintf("docker restart %s", shellEscape(containerName))
}

func DockerLogsCmd(containerName string) string {
	return fmt.Sprintf("docker logs --tail 200 %s", shellEscape(containerName))
}

// shellEscape wraps a string in single quotes for safe shell usage.
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
