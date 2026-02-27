package digitalocean

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/digitalocean/godo"
)

// CreateVolume creates a DigitalOcean block storage volume.
// Returns (volumeID, devicePath, error).
// Device path for DO volumes: /dev/disk/by-id/scsi-0DO_Volume_<name>
func CreateVolume(ctx context.Context, token, region, name string, sizeGB int) (string, string, error) {
	client := newClient(token)

	vol, _, err := client.Storage.CreateVolume(ctx, &godo.VolumeCreateRequest{
		Region:        region,
		Name:          name,
		SizeGigaBytes: int64(sizeGB),
		Description:   "localisprod managed block volume",
	})
	if err != nil {
		return "", "", fmt.Errorf("create DO volume: %w", err)
	}

	devicePath := "/dev/disk/by-id/scsi-0DO_Volume_" + name
	return vol.ID, devicePath, nil
}

// AttachVolume attaches a DigitalOcean volume to a Droplet and waits for the action to complete.
func AttachVolume(ctx context.Context, token, volumeID, dropletIDStr, region string) error {
	dropletID, err := strconv.Atoi(dropletIDStr)
	if err != nil {
		return fmt.Errorf("parse droplet ID %q: %w", dropletIDStr, err)
	}

	client := newClient(token)
	action, _, err := client.StorageActions.Attach(ctx, volumeID, dropletID)
	if err != nil {
		return fmt.Errorf("attach DO volume: %w", err)
	}

	return waitForAction(ctx, client, volumeID, action.ID)
}

// DetachVolume detaches a DigitalOcean volume from a Droplet and waits for the action to complete.
func DetachVolume(ctx context.Context, token, volumeID, dropletIDStr, region string) error {
	dropletID, err := strconv.Atoi(dropletIDStr)
	if err != nil {
		return fmt.Errorf("parse droplet ID %q: %w", dropletIDStr, err)
	}

	client := newClient(token)
	action, _, err := client.StorageActions.DetachByDropletID(ctx, volumeID, dropletID)
	if err != nil {
		return fmt.Errorf("detach DO volume: %w", err)
	}

	return waitForAction(ctx, client, volumeID, action.ID)
}

// DeleteVolume deletes a DigitalOcean block storage volume.
func DeleteVolume(ctx context.Context, token, volumeID string) error {
	client := newClient(token)
	_, err := client.Storage.DeleteVolume(ctx, volumeID)
	if err != nil {
		return fmt.Errorf("delete DO volume: %w", err)
	}
	return nil
}

// waitForAction polls until the action is completed or errored (max 5 minutes).
func waitForAction(ctx context.Context, client *godo.Client, volumeID string, actionID int) error {
	deadline := time.Now().Add(5 * time.Minute)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		action, _, err := client.StorageActions.Get(ctx, volumeID, actionID)
		if err != nil {
			return fmt.Errorf("poll action %d: %w", actionID, err)
		}
		switch action.Status {
		case "completed":
			return nil
		case "errored":
			return fmt.Errorf("DO action %d errored", actionID)
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("DO action %d did not complete within 5 minutes", actionID)
}
