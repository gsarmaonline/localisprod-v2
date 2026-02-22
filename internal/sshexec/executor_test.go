package sshexec_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gsarma/localisprod-v2/internal/models"
	"github.com/gsarma/localisprod-v2/internal/sshexec"
)

// ---- Command builder tests ----

func TestDockerLoginCmd(t *testing.T) {
	cmd := sshexec.DockerLoginCmd("myuser", "mytoken")
	if !strings.Contains(cmd, "docker login ghcr.io") {
		t.Errorf("expected docker login ghcr.io in command, got: %s", cmd)
	}
	if !strings.Contains(cmd, "myuser") {
		t.Errorf("expected username in command, got: %s", cmd)
	}
	if !strings.Contains(cmd, "--password-stdin") {
		t.Errorf("expected --password-stdin in command, got: %s", cmd)
	}
}

func TestDockerLoginCmd_ShellEscaping(t *testing.T) {
	// Tokens with single quotes must be safely escaped.
	cmd := sshexec.DockerLoginCmd("user", "tok'en")
	if strings.Count(cmd, "tok'en") > 0 {
		t.Errorf("unescaped single quote in token, command: %s", cmd)
	}
}

func TestDockerRunCmd_Minimal(t *testing.T) {
	cmd := sshexec.DockerRunCmd(sshexec.RunConfig{
		ContainerName: "my-container",
		Image:         "nginx:latest",
	})
	if !strings.HasPrefix(cmd, "docker run -d --name") {
		t.Errorf("expected docker run -d --name prefix, got: %s", cmd)
	}
	if !strings.Contains(cmd, "my-container") {
		t.Errorf("expected container name, got: %s", cmd)
	}
	if !strings.Contains(cmd, "nginx:latest") {
		t.Errorf("expected image name, got: %s", cmd)
	}
}

func TestDockerRunCmd_WithPorts(t *testing.T) {
	cmd := sshexec.DockerRunCmd(sshexec.RunConfig{
		ContainerName: "app",
		Image:         "myimage",
		Ports:         []string{"8080:80", "443:443"},
	})
	if !strings.Contains(cmd, "-p '8080:80'") {
		t.Errorf("expected first port mapping, got: %s", cmd)
	}
	if !strings.Contains(cmd, "-p '443:443'") {
		t.Errorf("expected second port mapping, got: %s", cmd)
	}
}

func TestDockerRunCmd_WithEnvFile(t *testing.T) {
	cmd := sshexec.DockerRunCmd(sshexec.RunConfig{
		ContainerName: "app",
		Image:         "myimage",
		EnvFilePath:   "/tmp/app.env",
	})
	if !strings.Contains(cmd, "--env-file") {
		t.Errorf("expected --env-file flag, got: %s", cmd)
	}
	if !strings.Contains(cmd, "/tmp/app.env") {
		t.Errorf("expected env file path, got: %s", cmd)
	}
}

func TestDockerRunCmd_NoEnvFile(t *testing.T) {
	cmd := sshexec.DockerRunCmd(sshexec.RunConfig{
		ContainerName: "app",
		Image:         "myimage",
		EnvFilePath:   "",
	})
	if strings.Contains(cmd, "--env-file") {
		t.Errorf("should not have --env-file when EnvFilePath is empty, got: %s", cmd)
	}
}

func TestDockerRunCmd_WithNetwork(t *testing.T) {
	cmd := sshexec.DockerRunCmd(sshexec.RunConfig{
		ContainerName: "app",
		Image:         "myimage",
		Network:       "traefik-net",
	})
	if !strings.Contains(cmd, "--network") {
		t.Errorf("expected --network flag, got: %s", cmd)
	}
	if !strings.Contains(cmd, "traefik-net") {
		t.Errorf("expected network name, got: %s", cmd)
	}
}

func TestDockerRunCmd_WithLabels(t *testing.T) {
	cmd := sshexec.DockerRunCmd(sshexec.RunConfig{
		ContainerName: "app",
		Image:         "myimage",
		Labels: map[string]string{
			"traefik.enable": "true",
		},
	})
	if !strings.Contains(cmd, "--label") {
		t.Errorf("expected --label flag, got: %s", cmd)
	}
	if !strings.Contains(cmd, "traefik.enable=true") {
		t.Errorf("expected label value, got: %s", cmd)
	}
}

func TestDockerRunCmd_WithCommand(t *testing.T) {
	cmd := sshexec.DockerRunCmd(sshexec.RunConfig{
		ContainerName: "app",
		Image:         "myimage",
		Command:       "serve --port 8080",
	})
	if !strings.Contains(cmd, "serve --port 8080") {
		t.Errorf("expected command appended, got: %s", cmd)
	}
}

func TestDockerRunCmd_ImageBeforeCommand(t *testing.T) {
	cmd := sshexec.DockerRunCmd(sshexec.RunConfig{
		ContainerName: "app",
		Image:         "myimage",
		Command:       "start",
	})
	imgIdx := strings.Index(cmd, "myimage")
	cmdIdx := strings.Index(cmd, "start")
	if imgIdx == -1 || cmdIdx == -1 {
		t.Fatal("image or command not found in output")
	}
	if imgIdx > cmdIdx {
		t.Errorf("image must appear before command in docker run, got: %s", cmd)
	}
}

func TestDockerPullCmd(t *testing.T) {
	cmd := sshexec.DockerPullCmd("ghcr.io/user/repo:latest")
	if !strings.HasPrefix(cmd, "docker pull") {
		t.Errorf("expected docker pull prefix, got: %s", cmd)
	}
	if !strings.Contains(cmd, "ghcr.io/user/repo:latest") {
		t.Errorf("expected image in command, got: %s", cmd)
	}
}

func TestDockerStopRemoveCmd(t *testing.T) {
	cmd := sshexec.DockerStopRemoveCmd("mycontainer")
	if !strings.Contains(cmd, "docker stop") {
		t.Errorf("expected docker stop, got: %s", cmd)
	}
	if !strings.Contains(cmd, "docker rm") {
		t.Errorf("expected docker rm, got: %s", cmd)
	}
	if !strings.Contains(cmd, "mycontainer") {
		t.Errorf("expected container name, got: %s", cmd)
	}
}

func TestDockerRestartCmd(t *testing.T) {
	cmd := sshexec.DockerRestartCmd("mycontainer")
	if !strings.HasPrefix(cmd, "docker restart") {
		t.Errorf("expected docker restart prefix, got: %s", cmd)
	}
	if !strings.Contains(cmd, "mycontainer") {
		t.Errorf("expected container name, got: %s", cmd)
	}
}

func TestDockerLogsCmd(t *testing.T) {
	cmd := sshexec.DockerLogsCmd("mycontainer")
	if !strings.HasPrefix(cmd, "docker logs") {
		t.Errorf("expected docker logs prefix, got: %s", cmd)
	}
	if !strings.Contains(cmd, "--tail") {
		t.Errorf("expected --tail flag, got: %s", cmd)
	}
	if !strings.Contains(cmd, "mycontainer") {
		t.Errorf("expected container name, got: %s", cmd)
	}
}

func TestRemoveFileCmd(t *testing.T) {
	cmd := sshexec.RemoveFileCmd("/tmp/myfile.env")
	if !strings.HasPrefix(cmd, "rm -f") {
		t.Errorf("expected rm -f prefix, got: %s", cmd)
	}
	if !strings.Contains(cmd, "/tmp/myfile.env") {
		t.Errorf("expected file path, got: %s", cmd)
	}
}

func TestTraefikSetupCmd(t *testing.T) {
	cmd := sshexec.TraefikSetupCmd()
	if !strings.Contains(cmd, "docker network create traefik-net") {
		t.Errorf("expected network creation, got: %s", cmd)
	}
	if !strings.Contains(cmd, "traefik:v3") {
		t.Errorf("expected traefik:v3 image, got: %s", cmd)
	}
	if !strings.Contains(cmd, "--providers.docker=true") {
		t.Errorf("expected docker provider flag, got: %s", cmd)
	}
	if !strings.Contains(cmd, "-p 80:80") {
		t.Errorf("expected port 80 published, got: %s", cmd)
	}
}

func TestTraefikLabels(t *testing.T) {
	labels := sshexec.TraefikLabels("myrouter", "example.com", "8080")
	if labels["traefik.enable"] != "true" {
		t.Errorf("expected traefik.enable=true, got: %v", labels["traefik.enable"])
	}
	if !strings.Contains(labels["traefik.http.routers.myrouter.rule"], "example.com") {
		t.Errorf("expected domain in router rule, got: %v", labels["traefik.http.routers.myrouter.rule"])
	}
	portKey := "traefik.http.services.myrouter.loadbalancer.server.port"
	if labels[portKey] != "8080" {
		t.Errorf("expected port 8080, got: %v", labels[portKey])
	}
	if labels["traefik.http.routers.myrouter.entrypoints"] != "web" {
		t.Errorf("expected entrypoints=web, got: %v", labels["traefik.http.routers.myrouter.entrypoints"])
	}
}

// ---- ShellEscape coverage via command outputs ----

func TestDockerRunCmd_ShellEscaping_SpecialChars(t *testing.T) {
	// Container name with hyphens should be safely quoted.
	cmd := sshexec.DockerRunCmd(sshexec.RunConfig{
		ContainerName: "my-app-abc12345",
		Image:         "ghcr.io/org/repo:latest",
	})
	if !strings.Contains(cmd, "'my-app-abc12345'") {
		t.Errorf("expected single-quoted container name, got: %s", cmd)
	}
}

// ---- NewRunner tests ----

func TestNewRunner_LocalNode_IsLocalRunner(t *testing.T) {
	node := &models.Node{IsLocal: true}
	runner := sshexec.NewRunner(node)
	// LocalRunner.Ping() should succeed without SSH.
	if err := runner.Ping(); err != nil {
		t.Errorf("LocalRunner.Ping() failed: %v", err)
	}
}

func TestNewRunner_RemoteNode_IsSSHClient(t *testing.T) {
	// Just verify it returns something (not nil); actual dial would fail.
	node := &models.Node{
		IsLocal:    false,
		Host:       "localhost",
		Port:       22,
		Username:   "testuser",
		PrivateKey: "invalid-key",
	}
	runner := sshexec.NewRunner(node)
	if runner == nil {
		t.Fatal("expected non-nil runner for remote node")
	}
}

// ---- LocalRunner integration tests ----

func TestLocalRunner_Ping(t *testing.T) {
	node := &models.Node{IsLocal: true}
	runner := sshexec.NewRunner(node)
	if err := runner.Ping(); err != nil {
		t.Errorf("Ping() unexpected error: %v", err)
	}
}

func TestLocalRunner_Run_Echo(t *testing.T) {
	node := &models.Node{IsLocal: true}
	runner := sshexec.NewRunner(node)
	out, err := runner.Run("echo hello")
	if err != nil {
		t.Fatalf("Run('echo hello') error: %v", err)
	}
	if out != "hello" {
		t.Errorf("expected 'hello', got %q", out)
	}
}

func TestLocalRunner_Run_TrimsWhitespace(t *testing.T) {
	node := &models.Node{IsLocal: true}
	runner := sshexec.NewRunner(node)
	out, err := runner.Run("printf '  hello  '")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "hello" {
		t.Errorf("expected trimmed output 'hello', got %q", out)
	}
}

func TestLocalRunner_Run_Error(t *testing.T) {
	node := &models.Node{IsLocal: true}
	runner := sshexec.NewRunner(node)
	_, err := runner.Run("exit 1")
	if err == nil {
		t.Error("expected error for non-zero exit code")
	}
}

func TestLocalRunner_WriteFile(t *testing.T) {
	node := &models.Node{IsLocal: true}
	runner := sshexec.NewRunner(node)

	dir := t.TempDir()
	path := filepath.Join(dir, "test.env")
	content := "FOO=bar\nBAR=baz\n"

	if err := runner.WriteFile(path, content); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if string(data) != content {
		t.Errorf("file content mismatch: got %q, want %q", string(data), content)
	}

	// Check file permissions are 0600.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("expected mode 0600, got %o", perm)
	}
}
