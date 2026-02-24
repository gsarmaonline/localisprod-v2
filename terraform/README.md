# Terraform — DigitalOcean Deployment

Provisions a DigitalOcean droplet and bootstraps localisprod via cloud-init.

## Prerequisites

- [Terraform](https://developer.hashicorp.com/terraform/install) >= 1.3
- [doctl](https://docs.digitalocean.com/reference/doctl/how-to/install/) (optional, for key fingerprints)
- A DigitalOcean API token
- An SSH key already added to your DigitalOcean account
- The Docker image published to GHCR (`push to main` triggers CI to publish it)

## Usage

```bash
# 1. Copy and fill in real values
cp terraform.tfvars.example terraform.tfvars

# 2. Get your SSH key fingerprint
doctl compute ssh-key list

# 3. Provision
terraform init
terraform plan
terraform apply

# 4. Note the outputs — you'll need the IP for GitHub Secrets
terraform output
```

## Continuous deployment

No additional GitHub secrets are needed. The CI workflow (`.github/workflows/ci.yml`) will:
1. Build and test on every push / PR
2. Build and push the Docker image to GHCR on every push to `main`

Watchtower running on the droplet checks GHCR every 60 seconds and automatically pulls and restarts the container when a new image is available.

## DNS setup

After `terraform apply`, point your domain's A record at the droplet IP:

```
localisprod.com  A  <droplet-ip>
```

Traefik will obtain a Let's Encrypt TLS certificate automatically on first request.

## Checking logs on the droplet

```bash
ssh root@<droplet-ip>
cd /opt/localisprod/app

# App logs
docker compose logs localisprod -f

# Traefik logs (proxy + TLS)
docker compose logs traefik -f

# All services
docker compose logs -f
```

## Variables

| Variable             | Default           | Description                                      |
|----------------------|-------------------|--------------------------------------------------|
| `do_token`           | *(required)*      | DigitalOcean API token                           |
| `ssh_key_fingerprint`| *(required)*      | Fingerprint of SSH key in your DO account        |
| `acme_email`         | *(required)*      | Email for Let's Encrypt certificate alerts       |
| `secret_key`         | `""`              | AES-256-GCM key (`openssl rand -base64 32`)      |
| `domain`             | `localisprod.com` | Domain to serve the app on                       |
| `region`             | `nyc3`            | DO region slug                                   |
| `droplet_size`       | `s-1vcpu-1gb`     | DO size slug                                     |
