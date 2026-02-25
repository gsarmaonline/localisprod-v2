package sshexec

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gsarma/localisprod-v2/internal/models"
	"golang.org/x/crypto/ssh"
)

// Runner abstracts over SSH and local execution.
type Runner interface {
	Run(cmd string) (string, error)
	WriteFile(path, content string) error
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

func (l *LocalRunner) WriteFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0600)
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

// WriteFile writes content to path on the remote host with mode 0600.
// Content is piped via stdin to avoid exposing it in the process list.
func (c *Client) WriteFile(path, content string) error {
	client, err := c.dial()
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	session.Stdin = strings.NewReader(content)
	// umask 177 ensures the file is created with mode 0600
	cmd := fmt.Sprintf("(umask 177 && cat > %s)", shellEscape(path))
	if out, err := session.CombinedOutput(cmd); err != nil {
		return fmt.Errorf("write file %s: %s: %w", path, strings.TrimSpace(string(out)), err)
	}
	return nil
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
	EnvFilePath   string            // path to --env-file on the node; "" = no env file
	Command       string
	Network       string            // "" = no --network flag
	Labels        map[string]string // arbitrary docker labels
	Volumes       []string          // "volume-name:/mount/path"
	Restart       string            // e.g. "unless-stopped"; "" = no --restart flag
}

// DockerRunCmd builds a docker run command. Env vars are passed via --env-file
// to avoid exposing values in the process list or shell history.
func DockerRunCmd(cfg RunConfig) string {
	var sb strings.Builder
	sb.WriteString("docker run -d --name ")
	sb.WriteString(shellEscape(cfg.ContainerName))

	if cfg.Restart != "" {
		sb.WriteString(" --restart ")
		sb.WriteString(shellEscape(cfg.Restart))
	}

	for _, p := range cfg.Ports {
		sb.WriteString(" -p ")
		sb.WriteString(shellEscape(p))
	}

	for _, v := range cfg.Volumes {
		sb.WriteString(" -v ")
		sb.WriteString(shellEscape(v))
	}

	if cfg.EnvFilePath != "" {
		sb.WriteString(" --env-file ")
		sb.WriteString(shellEscape(cfg.EnvFilePath))
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

// DockerVolumeCreateCmd returns a command to create a named Docker volume (idempotent).
func DockerVolumeCreateCmd(name string) string {
	return "docker volume create " + shellEscape(name)
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

// DockerInspectStatusCmd returns a command that prints the container's State.Status
// (e.g. "running", "exited", "paused"). Exits non-zero if the container doesn't exist.
func DockerInspectStatusCmd(containerName string) string {
	return fmt.Sprintf("docker inspect --format='{{.State.Status}}' %s", shellEscape(containerName))
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

// RemoveFileCmd returns a command to delete a file (best-effort, no error on missing).
func RemoveFileCmd(path string) string {
	return fmt.Sprintf("rm -f %s", shellEscape(path))
}

// CheckPortInUseCmd returns a shell command that prints the number of LISTEN
// entries for the given port.  The command exits non-zero when the port is free
// (grep found no matches), which IsPortInUse treats as "not in use".  Removing
// the trailing "|| echo 0" avoids a double-output bug on systems where ss is
// unavailable: grep -c would exit 1 (no matches) triggering the fallback and
// producing "0\n0" instead of "0".
func CheckPortInUseCmd(port int) string {
	return fmt.Sprintf("ss -tln sport = :%d 2>/dev/null | grep -c LISTEN", port)
}

// IsPortInUse runs CheckPortInUseCmd on the given runner and returns true if
// the port is already listening. Errors from the runner are silently ignored so
// a missing ss binary does not block container creation.
func IsPortInUse(runner Runner, port int) (bool, error) {
	out, err := runner.Run(CheckPortInUseCmd(port))
	if err != nil {
		return false, nil
	}
	return strings.TrimSpace(out) != "0", nil
}

// ShellEscape wraps a string in single quotes for safe shell usage.
func ShellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func shellEscape(s string) string { return ShellEscape(s) }
