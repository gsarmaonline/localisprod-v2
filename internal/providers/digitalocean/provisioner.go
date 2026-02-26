package digitalocean

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/digitalocean/godo"
	"golang.org/x/crypto/ssh"
)

// Region represents a DigitalOcean region.
type Region struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// Size represents a DigitalOcean Droplet size.
type Size struct {
	Slug        string  `json:"slug"`
	Description string  `json:"description"`
	VCPUs       int     `json:"vcpus"`
	MemoryMB    int     `json:"memory_mb"`
	DiskGB      int     `json:"disk_gb"`
	PriceMonthly float64 `json:"price_monthly"`
}

// Image represents a DigitalOcean distribution image.
type Image struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

func newClient(token string) *godo.Client {
	return godo.NewFromToken(token)
}

// ValidateToken checks that the DigitalOcean API token is valid by fetching
// basic account info. Returns a descriptive error if the token is rejected.
func ValidateToken(ctx context.Context, token string) error {
	client := newClient(token)
	_, _, err := client.Account.Get(ctx)
	if err != nil {
		return fmt.Errorf("DigitalOcean token validation failed: %w", err)
	}
	return nil
}

// ListRegions returns available DigitalOcean regions.
func ListRegions(ctx context.Context, token string) ([]Region, error) {
	client := newClient(token)
	regions, _, err := client.Regions.List(ctx, &godo.ListOptions{PerPage: 200})
	if err != nil {
		return nil, fmt.Errorf("list regions: %w", err)
	}
	var out []Region
	for _, r := range regions {
		if r.Available {
			out = append(out, Region{Slug: r.Slug, Name: r.Name})
		}
	}
	return out, nil
}

// ListSizes returns curated DigitalOcean Droplet sizes (shared and general-purpose CPU).
func ListSizes(ctx context.Context, token string) ([]Size, error) {
	client := newClient(token)
	sizes, _, err := client.Sizes.List(ctx, &godo.ListOptions{PerPage: 200})
	if err != nil {
		return nil, fmt.Errorf("list sizes: %w", err)
	}
	var out []Size
	for _, s := range sizes {
		if !s.Available {
			continue
		}
		// Only include shared CPU (s-*) and general purpose (g-*) slugs
		if len(s.Slug) < 2 {
			continue
		}
		prefix := s.Slug[:2]
		if prefix != "s-" && prefix != "g-" {
			continue
		}
		desc := fmt.Sprintf("%d vCPU / %d MB RAM / %d GB SSD", s.Vcpus, s.Memory, s.Disk)
		out = append(out, Size{
			Slug:         s.Slug,
			Description:  desc,
			VCPUs:        s.Vcpus,
			MemoryMB:     s.Memory,
			DiskGB:       s.Disk,
			PriceMonthly: float64(s.PriceMonthly),
		})
	}
	return out, nil
}

// ListImages returns curated distribution images (Ubuntu 22.04, Ubuntu 24.04, Debian 12).
func ListImages(ctx context.Context, token string) ([]Image, error) {
	client := newClient(token)
	images, _, err := client.Images.ListDistribution(ctx, &godo.ListOptions{PerPage: 200})
	if err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}
	allowed := map[string]bool{
		"ubuntu-22-04-x64": true,
		"ubuntu-24-04-x64": true,
		"debian-12-x64":    true,
	}
	var out []Image
	for _, img := range images {
		if img.Slug != "" && allowed[img.Slug] {
			out = append(out, Image{Slug: img.Slug, Name: img.Name})
		}
	}
	// If API didn't return slugs, provide curated static list
	if len(out) == 0 {
		out = []Image{
			{Slug: "ubuntu-22-04-x64", Name: "Ubuntu 22.04 (LTS) x64"},
			{Slug: "ubuntu-24-04-x64", Name: "Ubuntu 24.04 (LTS) x64"},
			{Slug: "debian-12-x64", Name: "Debian 12 x64"},
		}
	}
	return out, nil
}

// generateED25519KeyPair generates an ed25519 key pair and returns
// (privateKeyPEM, authorizedKeyLine, error).
func generateED25519KeyPair() (privateKeyPEM string, authorizedKey string, err error) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("generate ed25519 key: %w", err)
	}

	// Encode private key to PEM (OpenSSH format)
	privPEM, err := ssh.MarshalPrivateKey(privKey, "")
	if err != nil {
		return "", "", fmt.Errorf("marshal private key: %w", err)
	}
	privateKeyPEM = string(pem.EncodeToMemory(privPEM))

	// Encode public key to authorized_keys format
	sshPub, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return "", "", fmt.Errorf("create ssh public key: %w", err)
	}
	authorizedKey = string(ssh.MarshalAuthorizedKey(sshPub))
	return privateKeyPEM, authorizedKey, nil
}

// ProvisionDroplet creates a new DigitalOcean Droplet, waits for it to be active,
// and returns (host IP, droplet ID, private key PEM, error).
func ProvisionDroplet(ctx context.Context, token, region, size, image, name string) (host, instanceID, privateKeyPEM string, err error) {
	client := newClient(token)

	privPEM, pubKey, err := generateED25519KeyPair()
	if err != nil {
		return "", "", "", err
	}

	// Upload public key to DO
	sshKeyReq := &godo.KeyCreateRequest{
		Name:      "localisprod-" + name + "-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		PublicKey: pubKey,
	}
	doKey, _, err := client.Keys.Create(ctx, sshKeyReq)
	if err != nil {
		return "", "", "", fmt.Errorf("create ssh key: %w", err)
	}
	// Cleanup key resource after we're done
	defer func() {
		_, _ = client.Keys.DeleteByID(context.Background(), doKey.ID)
	}()

	// Create droplet
	createReq := &godo.DropletCreateRequest{
		Name:   name,
		Region: region,
		Size:   size,
		Image: godo.DropletCreateImage{
			Slug: image,
		},
		SSHKeys: []godo.DropletCreateSSHKey{
			{ID: doKey.ID},
		},
	}
	droplet, _, err := client.Droplets.Create(ctx, createReq)
	if err != nil {
		return "", "", "", fmt.Errorf("create droplet: %w", err)
	}

	dropletID := droplet.ID
	instanceIDStr := fmt.Sprintf("%d", dropletID)

	// Poll until active (max 5 minutes)
	deadline := time.Now().Add(5 * time.Minute)
	var publicIP string
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return "", "", "", ctx.Err()
		default:
		}

		d, _, err := client.Droplets.Get(ctx, dropletID)
		if err != nil {
			return "", "", "", fmt.Errorf("get droplet status: %w", err)
		}
		if d.Status == "active" {
			for _, net := range d.Networks.V4 {
				if net.Type == "public" {
					publicIP = net.IPAddress
					break
				}
			}
			if publicIP != "" {
				break
			}
		}
		time.Sleep(5 * time.Second)
	}

	if publicIP == "" {
		return "", "", "", fmt.Errorf("droplet did not become active within 5 minutes")
	}

	return publicIP, instanceIDStr, privPEM, nil
}

// DefaultUsername returns the default SSH username for a given DigitalOcean image slug.
func DefaultUsername(imageSlug string) string {
	switch imageSlug {
	case "debian-12-x64":
		return "root"
	default:
		// Ubuntu images use "root" by default on DigitalOcean
		return "root"
	}
}
