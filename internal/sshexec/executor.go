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

// RunConfig holds parameters for docker run.
type RunConfig struct {
	ContainerName string
	Image         string
	Ports         []string
	EnvVars       map[string]string
	Command       string
	Network       string            // "" = no --network flag
	Labels        map[string]string // arbitrary docker labels
}

func DockerRunCmd(cfg RunConfig) string {
	var sb strings.Builder
	sb.WriteString("docker run -d --name ")
	sb.WriteString(shellEscape(cfg.ContainerName))

	for _, p := range cfg.Ports {
		sb.WriteString(" -p ")
		sb.WriteString(shellEscape(p))
	}

	for k, v := range cfg.EnvVars {
		sb.WriteString(" -e ")
		sb.WriteString(shellEscape(k + "=" + v))
	}

	if cfg.Network != "" {
		sb.WriteString(" --network ")
		sb.WriteString(shellEscape(cfg.Network))
	}

	for k, v := range cfg.Labels {
		sb.WriteString(" --label ")
		sb.WriteString(shellEscape(k + "=" + v))
	}

	sb.WriteString(" ")
	sb.WriteString(shellEscape(cfg.Image))

	if cfg.Command != "" {
		sb.WriteString(" ")
		sb.WriteString(cfg.Command)
	}

	return sb.String()
}

// TraefikSetupCmd returns a shell command that installs Traefik on a node.
func TraefikSetupCmd() string {
	return strings.Join([]string{
		"docker network create traefik-net 2>/dev/null || true",
		"docker stop traefik 2>/dev/null || true && docker rm traefik 2>/dev/null || true",
		"docker run -d --name traefik --restart unless-stopped" +
			" -p 80:80" +
			" -v /var/run/docker.sock:/var/run/docker.sock:ro" +
			" --network traefik-net" +
			" traefik:v3" +
			" --providers.docker=true" +
			" --providers.docker.exposedbydefault=false" +
			" --providers.docker.network=traefik-net" +
			" --entrypoints.web.address=:80",
	}, " && ")
}

// TraefikLabels returns the docker labels needed for Traefik to route to a container.
func TraefikLabels(routerName, domain, containerPort string) map[string]string {
	return map[string]string{
		"traefik.enable": "true",
		fmt.Sprintf("traefik.http.routers.%s.rule", routerName):                      fmt.Sprintf("Host(`%s`)", domain),
		fmt.Sprintf("traefik.http.routers.%s.entrypoints", routerName):               "web",
		fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port", routerName): containerPort,
	}
}

func DockerPullCmd(image string) string {
	return "docker pull " + shellEscape(image)
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
