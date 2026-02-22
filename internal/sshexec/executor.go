package sshexec

import (
	"fmt"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type Client struct {
	host       string
	port       int
	username   string
	privateKey string
}

func New(host string, port int, username, privateKey string) *Client {
	return &Client{
		host:       host,
		port:       port,
		username:   username,
		privateKey: privateKey,
	}
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
