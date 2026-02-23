# Terraform — DigitalOcean Deployment

Provisions a DigitalOcean droplet and bootstraps localisprod via cloud-init.

## Prerequisites

- [Terraform](https://developer.hashicorp.com/terraform/install) >= 1.3
- [doctl](https://docs.digitalocean.com/reference/doctl/how-to/install/) (optional, for key fingerprints)
- A DigitalOcean API token
- An SSH key already added to your DigitalOcean account

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

## GitHub Actions secrets

After `terraform apply`, set these in **GitHub → Settings → Secrets and variables → Actions**:

| Secret             | Value                                          |
|--------------------|------------------------------------------------|
| `DO_DROPLET_IP`    | IP from `terraform output droplet_ip`          |
| `DEPLOY_SSH_KEY`   | Private key whose public key is on the droplet |

The CI workflow (`.github/workflows/ci.yml`) will then:
1. Build and test on every push / PR
2. SSH into the droplet and redeploy automatically on every push to `main`

## DNS setup

After `terraform apply`, point your domain's A record at the droplet IP:

```
localisprod.com  A  <droplet-ip>
```

Traefik will obtain a Let's Encrypt TLS certificate automatically on first request.

## Checking logs on the droplet

```bash
ssh root@<droplet-ip>

# App logs
journalctl -u localisprod -f

# Traefik logs (proxy + TLS)
journalctl -u traefik -f
```

## Variables

| Variable             | Default           | Description                                      |
|----------------------|-------------------|--------------------------------------------------|
| `do_token`           | *(required)*      | DigitalOcean API token                           |
| `ssh_key_fingerprint`| *(required)*      | Fingerprint of SSH key in your DO account        |
| `repo_url`           | *(required)*      | Git clone URL for this repo                      |
| `acme_email`         | *(required)*      | Email for Let's Encrypt certificate alerts       |
| `secret_key`         | `""`              | AES-256-GCM key (`openssl rand -base64 32`)      |
| `domain`             | `localisprod.com` | Domain to serve the app on                       |
| `traefik_version`    | `v3.2.11`         | Traefik release version to install               |
| `region`             | `nyc3`            | DO region slug                                   |
| `droplet_size`       | `s-1vcpu-1gb`     | DO size slug                                     |
| `port`               | `8080`            | Internal HTTP port (not publicly exposed)        |
