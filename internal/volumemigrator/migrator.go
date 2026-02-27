package volumemigrator

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/gsarma/localisprod-v2/internal/models"
	awsvol "github.com/gsarma/localisprod-v2/internal/providers/aws"
	dovol "github.com/gsarma/localisprod-v2/internal/providers/digitalocean"
	"github.com/gsarma/localisprod-v2/internal/sshexec"
	"github.com/gsarma/localisprod-v2/internal/store"
)

const (
	defaultVolumeSizeGB = 20
	volumesDir          = "/var/lib/docker/volumes"
	volumesBakDir       = "/var/lib/docker/volumes.bak"
)

// Migrator orchestrates block-storage volume migrations for nodes.
type Migrator struct {
	store *store.Store
}

// New creates a new Migrator.
func New(s *store.Store) *Migrator {
	return &Migrator{store: s}
}

// Migrate runs the full migration flow asynchronously.
// The migration record must already exist in the DB with status "pending".
func (m *Migrator) Migrate(node *models.Node, migration *models.NodeVolumeMigration, userID string) {
	go func() {
		ctx := context.Background()
		if err := m.runMigrate(ctx, node, migration, userID); err != nil {
			log.Printf("volumemigrator: migration %s failed: %v", migration.ID, err)
		}
	}()
}

// Rollback runs the rollback flow asynchronously.
func (m *Migrator) Rollback(node *models.Node, migration *models.NodeVolumeMigration, userID string) {
	go func() {
		ctx := context.Background()
		if err := m.runRollback(ctx, node, migration, userID); err != nil {
			log.Printf("volumemigrator: rollback %s failed: %v", migration.ID, err)
		}
	}()
}

func (m *Migrator) setStatus(id, status, errMsg string) {
	if err := m.store.UpdateVolumeMigrationStatus(id, status, errMsg); err != nil {
		log.Printf("volumemigrator: update status %s->%s: %v", id, status, err)
	}
}

func (m *Migrator) runMigrate(ctx context.Context, node *models.Node, migration *models.NodeVolumeMigration, userID string) error {
	runner := sshexec.NewRunner(node)

	// Step 1: Create provider block volume
	m.setStatus(migration.ID, "provisioning", "")
	volumeID, devicePath, err := m.createProviderVolume(ctx, node, userID, migration.ID)
	if err != nil {
		m.setStatus(migration.ID, "failed", fmt.Sprintf("create volume: %s", err))
		return err
	}
	if err := m.store.UpdateVolumeMigrationProviderVolume(migration.ID, volumeID, devicePath); err != nil {
		log.Printf("volumemigrator: persist volume ID: %v", err)
	}

	// Step 2: Attach volume to instance
	m.setStatus(migration.ID, "provisioned", "")
	if err := m.attachProviderVolume(ctx, node, userID, volumeID); err != nil {
		m.setStatus(migration.ID, "failed", fmt.Sprintf("attach volume: %s", err))
		m.cleanupVolume(ctx, node, userID, volumeID)
		return err
	}

	// Step 3: mkfs + mount
	m.setStatus(migration.ID, "mounted", "")
	mountPath := migration.MountPath
	if _, err := runner.Run(sshexec.MkfsAndMountCmd(devicePath, mountPath)); err != nil {
		m.setStatus(migration.ID, "failed", fmt.Sprintf("mkfs/mount: %s", err))
		m.runDetachDelete(ctx, node, userID, volumeID)
		return err
	}

	// Step 4: rsync /var/lib/docker/volumes/ → mountPath/volumes/
	m.setStatus(migration.ID, "synced", "")
	dstVolumes := mountPath + "/volumes/"
	if _, err := runner.Run(sshexec.RsyncVolumesCmd(volumesDir+"/", dstVolumes)); err != nil {
		m.setStatus(migration.ID, "failed", fmt.Sprintf("rsync: %s", err))
		m.runDetachDelete(ctx, node, userID, volumeID)
		return err
	}

	// Collect container names for stop/start
	containers, err := m.store.ListContainerNamesByNodeID(node.ID, userID)
	if err != nil {
		containers = nil // proceed without stopping if we can't list
		log.Printf("volumemigrator: list containers: %v", err)
	}

	// Step 5: Stop all containers
	m.setStatus(migration.ID, "stopping", "")
	if _, err := runner.Run(sshexec.StopContainersCmd(containers)); err != nil {
		m.setStatus(migration.ID, "failed", fmt.Sprintf("stop containers: %s", err))
		m.runDetachDelete(ctx, node, userID, volumeID)
		return err
	}

	// Step 6: Rename /var/lib/docker/volumes → /var/lib/docker/volumes.bak
	m.setStatus(migration.ID, "renamed", "")
	if _, err := runner.Run(sshexec.RenameDirCmd(volumesDir, volumesBakDir)); err != nil {
		m.setStatus(migration.ID, "failed", fmt.Sprintf("rename volumes dir: %s", err))
		// Restart containers before reporting failure
		_, _ = runner.Run(sshexec.StartContainersCmd(containers))
		m.runDetachDelete(ctx, node, userID, volumeID)
		return err
	}

	// Step 7: Symlink /var/lib/docker/volumes → mountPath/volumes
	m.setStatus(migration.ID, "symlinked", "")
	if _, err := runner.Run(sshexec.CreateSymlinkCmd(dstVolumes, volumesDir)); err != nil {
		m.setStatus(migration.ID, "failed", fmt.Sprintf("create symlink: %s", err))
		// Attempt to restore .bak before reporting failure
		_, _ = runner.Run(sshexec.RenameDirCmd(volumesBakDir, volumesDir))
		_, _ = runner.Run(sshexec.StartContainersCmd(containers))
		m.runDetachDelete(ctx, node, userID, volumeID)
		return err
	}

	// Step 8: Restart containers
	m.setStatus(migration.ID, "restarting", "")
	if _, err := runner.Run(sshexec.StartContainersCmd(containers)); err != nil {
		m.setStatus(migration.ID, "failed", fmt.Sprintf("start containers: %s", err))
		return err
	}

	// Step 9: Verify containers are running
	m.setStatus(migration.ID, "verified", "")
	if len(containers) > 0 {
		out, err := runner.Run(sshexec.CheckContainersHealthCmd(containers))
		if err != nil || strings.Contains(out, "false") {
			m.setStatus(migration.ID, "failed", fmt.Sprintf("health check failed: %s", out))
			// Trigger rollback
			current, _ := m.store.GetVolumeMigration(node.ID)
			if current != nil {
				_ = m.runRollback(ctx, node, current, userID)
			}
			return fmt.Errorf("container health check failed after migration")
		}
	}

	// Step 10: Completed
	m.setStatus(migration.ID, "completed", "")
	return nil
}

func (m *Migrator) runRollback(ctx context.Context, node *models.Node, migration *models.NodeVolumeMigration, userID string) error {
	m.setStatus(migration.ID, "rolling_back", "")
	runner := sshexec.NewRunner(node)

	containers, err := m.store.ListContainerNamesByNodeID(node.ID, userID)
	if err != nil {
		containers = nil
	}

	// Step R1: Stop containers
	_, _ = runner.Run(sshexec.StopContainersCmd(containers))

	// Step R2: Remove symlink at /var/lib/docker/volumes (if symlink)
	_, _ = runner.Run(sshexec.RemoveSymlinkCmd(volumesDir))

	// Step R3: Rename .bak back to volumes (if .bak exists)
	_, _ = runner.Run(fmt.Sprintf("test -d %s && mv %s %s || true",
		sshexec.ShellEscape(volumesBakDir),
		sshexec.ShellEscape(volumesBakDir),
		sshexec.ShellEscape(volumesDir),
	))

	// Step R4: Restart containers
	_, _ = runner.Run(sshexec.StartContainersCmd(containers))

	// Step R5: Detach + delete provider volume
	if migration.ProviderVolumeID != "" {
		m.runDetachDelete(ctx, node, userID, migration.ProviderVolumeID)
	}

	m.setStatus(migration.ID, "rolled_back", "")
	return nil
}

// createProviderVolume creates the block volume for the node's provider.
// Returns (volumeID, devicePath).
func (m *Migrator) createProviderVolume(ctx context.Context, node *models.Node, userID, migrationID string) (string, string, error) {
	region := node.ProviderRegion
	instanceID := node.ProviderInstanceID
	volumeName := "localis-" + node.ID[:8]

	switch node.Provider {
	case "aws":
		accessKey, err := m.store.GetSecretUserSetting(userID, "aws_access_key_id")
		if err != nil || accessKey == "" {
			return "", "", fmt.Errorf("AWS access key not configured")
		}
		secretKey, err := m.store.GetSecretUserSetting(userID, "aws_secret_access_key")
		if err != nil || secretKey == "" {
			return "", "", fmt.Errorf("AWS secret key not configured")
		}
		return awsvol.CreateVolume(ctx, accessKey, secretKey, region, instanceID, defaultVolumeSizeGB)

	case "digitalocean":
		token, err := m.store.GetSecretUserSetting(userID, "do_api_token")
		if err != nil || token == "" {
			return "", "", fmt.Errorf("DigitalOcean API token not configured")
		}
		return dovol.CreateVolume(ctx, token, region, volumeName, defaultVolumeSizeGB)

	default:
		return "", "", fmt.Errorf("unsupported provider %q", node.Provider)
	}
}

// attachProviderVolume attaches volumeID to the node's instance.
func (m *Migrator) attachProviderVolume(ctx context.Context, node *models.Node, userID, volumeID string) error {
	switch node.Provider {
	case "aws":
		accessKey, err := m.store.GetSecretUserSetting(userID, "aws_access_key_id")
		if err != nil || accessKey == "" {
			return fmt.Errorf("AWS access key not configured")
		}
		secretKey, err := m.store.GetSecretUserSetting(userID, "aws_secret_access_key")
		if err != nil || secretKey == "" {
			return fmt.Errorf("AWS secret key not configured")
		}
		return awsvol.AttachVolume(ctx, accessKey, secretKey, node.ProviderRegion, volumeID, node.ProviderInstanceID)

	case "digitalocean":
		token, err := m.store.GetSecretUserSetting(userID, "do_api_token")
		if err != nil || token == "" {
			return fmt.Errorf("DigitalOcean API token not configured")
		}
		return dovol.AttachVolume(ctx, token, volumeID, node.ProviderInstanceID, node.ProviderRegion)

	default:
		return fmt.Errorf("unsupported provider %q", node.Provider)
	}
}

// runDetachDelete detaches and deletes the provider volume (best-effort, errors logged).
func (m *Migrator) runDetachDelete(ctx context.Context, node *models.Node, userID, volumeID string) {
	if err := m.detachProviderVolume(ctx, node, userID, volumeID); err != nil {
		log.Printf("volumemigrator: detach volume %s: %v", volumeID, err)
	}
	if err := m.deleteProviderVolume(ctx, node, userID, volumeID); err != nil {
		log.Printf("volumemigrator: delete volume %s: %v", volumeID, err)
	}
}

func (m *Migrator) detachProviderVolume(ctx context.Context, node *models.Node, userID, volumeID string) error {
	switch node.Provider {
	case "aws":
		accessKey, _ := m.store.GetSecretUserSetting(userID, "aws_access_key_id")
		secretKey, _ := m.store.GetSecretUserSetting(userID, "aws_secret_access_key")
		return awsvol.DetachVolume(ctx, accessKey, secretKey, node.ProviderRegion, volumeID)
	case "digitalocean":
		token, _ := m.store.GetSecretUserSetting(userID, "do_api_token")
		return dovol.DetachVolume(ctx, token, volumeID, node.ProviderInstanceID, node.ProviderRegion)
	default:
		return fmt.Errorf("unsupported provider %q", node.Provider)
	}
}

func (m *Migrator) deleteProviderVolume(ctx context.Context, node *models.Node, userID, volumeID string) error {
	switch node.Provider {
	case "aws":
		accessKey, _ := m.store.GetSecretUserSetting(userID, "aws_access_key_id")
		secretKey, _ := m.store.GetSecretUserSetting(userID, "aws_secret_access_key")
		return awsvol.DeleteVolume(ctx, accessKey, secretKey, node.ProviderRegion, volumeID)
	case "digitalocean":
		token, _ := m.store.GetSecretUserSetting(userID, "do_api_token")
		return dovol.DeleteVolume(ctx, token, volumeID)
	default:
		return fmt.Errorf("unsupported provider %q", node.Provider)
	}
}

// cleanupVolume deletes a volume that was created but never attached (e.g. if attach failed).
func (m *Migrator) cleanupVolume(ctx context.Context, node *models.Node, userID, volumeID string) {
	if err := m.deleteProviderVolume(ctx, node, userID, volumeID); err != nil {
		log.Printf("volumemigrator: cleanup volume %s: %v", volumeID, err)
	}
}
