package poller

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/gsarma/localisprod-v2/internal/sshexec"
	"github.com/gsarma/localisprod-v2/internal/store"
)

// Poller runs two background loops:
//   - image check (interval): pulls each running deployment's image; redeploys if newer
//   - health check (statusInterval): pings nodes and docker-inspects containers to keep
//     status fields accurate in the database
type Poller struct {
	store          *store.Store
	interval       time.Duration
	statusInterval time.Duration
}

func New(s *store.Store, interval, statusInterval time.Duration) *Poller {
	return &Poller{store: s, interval: interval, statusInterval: statusInterval}
}

// Start runs both loops until ctx is cancelled.
func (p *Poller) Start(ctx context.Context) {
	log.Printf("poller: image check every %s, health check every %s", p.interval, p.statusInterval)

	imageTicker := time.NewTicker(p.interval)
	statusTicker := time.NewTicker(p.statusInterval)
	defer imageTicker.Stop()
	defer statusTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-imageTicker.C:
			p.checkImages()
		case <-statusTicker.C:
			p.reconcileStatus()
		}
	}
}

// checkImages pulls the image for every running deployment and redeploys when a
// newer image has been downloaded.
func (p *Poller) checkImages() {
	deployments, err := p.store.ListAllRunningDeployments()
	if err != nil {
		log.Printf("poller: list running deployments: %v", err)
		return
	}

	for _, d := range deployments {
		app, err := p.store.GetApplication(d.ApplicationID, d.UserID)
		if err != nil || app == nil {
			log.Printf("poller: get application %s: %v", d.ApplicationID, err)
			continue
		}

		node, err := p.store.GetNode(d.NodeID, d.UserID)
		if err != nil || node == nil {
			log.Printf("poller: get node %s: %v", d.NodeID, err)
			continue
		}

		ghToken, _ := p.store.GetUserSetting(d.UserID, "github_token")
		ghUsername, _ := p.store.GetUserSetting(d.UserID, "github_username")

		runner := sshexec.NewRunner(node)

		if strings.HasPrefix(app.DockerImage, "ghcr.io/") && ghToken != "" && ghUsername != "" {
			loginCmd := sshexec.DockerLoginCmd(ghUsername, ghToken)
			if _, loginErr := runner.Run(loginCmd); loginErr != nil {
				log.Printf("poller: docker login failed for deployment %s: %v", d.ID, loginErr)
				continue
			}
		}

		pullOutput, pullErr := runner.Run(sshexec.DockerPullCmd(app.DockerImage))
		if pullErr != nil {
			log.Printf("poller: docker pull failed for deployment %s (%s): %v", d.ID, app.DockerImage, pullErr)
			continue
		}
		if !strings.Contains(pullOutput, "Downloaded newer image") {
			continue // already up to date
		}

		log.Printf("poller: new image for %s (%s), redeploying deployment %s", app.Name, app.DockerImage, d.ID)

		if _, stopErr := runner.Run(sshexec.DockerStopRemoveCmd(d.ContainerName)); stopErr != nil {
			log.Printf("poller: stop/rm failed for deployment %s: %v", d.ID, stopErr)
		}

		var ports []string
		_ = json.Unmarshal([]byte(app.Ports), &ports)

		cfg := sshexec.RunConfig{
			ContainerName: d.ContainerName,
			Image:         app.DockerImage,
			Ports:         ports,
			Command:       app.Command,
		}
		if app.Domain != "" {
			containerPort := "80"
			if len(ports) > 0 {
				if idx := strings.LastIndex(ports[0], ":"); idx >= 0 {
					containerPort = ports[0][idx+1:]
				}
			}
			cfg.Network = "traefik-net"
			cfg.Labels = sshexec.TraefikLabels(d.ContainerName, app.Domain, containerPort)
		}

		runOutput, runErr := runner.Run(sshexec.DockerRunCmd(cfg))
		if runErr != nil {
			log.Printf("poller: docker run failed for deployment %s: %v\noutput: %s", d.ID, runErr, runOutput)
			_ = p.store.UpdateDeploymentStatus(d.ID, d.UserID, "failed", "")
			continue
		}

		newContainerID := strings.TrimSpace(runOutput)
		_ = p.store.UpdateDeploymentStatus(d.ID, d.UserID, "running", newContainerID)
		log.Printf("poller: redeployed %s → container %s", d.ContainerName, newContainerID)
	}
}

// reconcileStatus pings every node and docker-inspects every running container,
// updating status in the database when reality differs from what's stored.
func (p *Poller) reconcileStatus() {
	p.reconcileNodes()
	p.reconcileDeployments()
	p.reconcileDatabases()
}

func (p *Poller) reconcileNodes() {
	nodes, err := p.store.ListAllNodes()
	if err != nil {
		log.Printf("poller: list nodes: %v", err)
		return
	}
	for _, n := range nodes {
		runner := sshexec.NewRunner(n)
		want := "online"
		if err := runner.Ping(); err != nil {
			want = "offline"
		}
		if n.Status != want {
			_ = p.store.UpdateNodeStatus(n.ID, n.UserID, want)
			log.Printf("poller: node %s status %s → %s", n.Name, n.Status, want)
		}
	}
}

func (p *Poller) reconcileDeployments() {
	deployments, err := p.store.ListAllRunningDeployments()
	if err != nil {
		log.Printf("poller: list running deployments for health check: %v", err)
		return
	}
	for _, d := range deployments {
		node, err := p.store.GetNode(d.NodeID, d.UserID)
		if err != nil || node == nil {
			continue
		}
		runner := sshexec.NewRunner(node)
		output, err := runner.Run(sshexec.DockerInspectStatusCmd(d.ContainerName))
		if err != nil {
			// SSH failure or container not found — don't flip status on transient errors
			log.Printf("poller: inspect deployment %s (%s): %v", d.ID, d.ContainerName, err)
			continue
		}
		if strings.TrimSpace(output) != "running" {
			_ = p.store.UpdateDeploymentStatus(d.ID, d.UserID, "stopped", "")
			log.Printf("poller: deployment %s container %s is %q, marked stopped", d.ID, d.ContainerName, strings.TrimSpace(output))
		}
	}
}

func (p *Poller) reconcileDatabases() {
	dbs, err := p.store.ListAllRunningDatabases()
	if err != nil {
		log.Printf("poller: list running databases for health check: %v", err)
		return
	}
	for _, db := range dbs {
		node, err := p.store.GetNode(db.NodeID, db.UserID)
		if err != nil || node == nil {
			continue
		}
		runner := sshexec.NewRunner(node)
		output, err := runner.Run(sshexec.DockerInspectStatusCmd(db.ContainerName))
		if err != nil {
			log.Printf("poller: inspect database %s (%s): %v", db.ID, db.ContainerName, err)
			continue
		}
		if strings.TrimSpace(output) != "running" {
			_ = p.store.UpdateDatabaseStatus(db.ID, db.UserID, "stopped")
			log.Printf("poller: database %s container %s is %q, marked stopped", db.ID, db.ContainerName, strings.TrimSpace(output))
		}
	}
}
