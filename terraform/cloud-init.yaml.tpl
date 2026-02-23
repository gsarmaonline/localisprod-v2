#cloud-config

package_update: true
package_upgrade: true
packages:
  - git
  - curl

runcmd:
  # -- Swap (essential for 1 GB droplets: Docker image builds can OOM) ----------
  - fallocate -l 2G /swapfile
  - chmod 600 /swapfile
  - mkswap /swapfile
  - swapon /swapfile
  - echo '/swapfile none swap sw 0 0' >> /etc/fstab

  # -- Docker -------------------------------------------------------------------
  - curl -fsSL https://get.docker.com | sh
  - systemctl enable docker
  - systemctl start docker

  # -- GitHub deploy key (needed for private repos) -----------------------------
  %{ if github_deploy_key != "" ~}
  - mkdir -p /root/.ssh
  - chmod 700 /root/.ssh
  - |
    cat > /root/.ssh/github_deploy_key <<'KEYEOF'
    ${github_deploy_key}
    KEYEOF
  - chmod 600 /root/.ssh/github_deploy_key
  - |
    cat >> /root/.ssh/config <<'SSHEOF'
    Host github.com
      IdentityFile /root/.ssh/github_deploy_key
      StrictHostKeyChecking no
    SSHEOF
  %{ endif ~}

  # -- Clone repo ---------------------------------------------------------------
  - git clone ${repo_url} /opt/localisprod/app

  # -- Write .env ---------------------------------------------------------------
  - |
    cat > /opt/localisprod/app/.env <<'ENVEOF'
    PORT=8080
    DB_PATH=/data/cluster.db
    SECRET_KEY=${secret_key}
    JWT_SECRET=${jwt_secret}
    GOOGLE_CLIENT_ID=${google_client_id}
    GOOGLE_CLIENT_SECRET=${google_client_secret}
    APP_URL=https://${domain}
    DOMAIN=${domain}
    ACME_EMAIL=${acme_email}
    ENVEOF
  - chmod 600 /opt/localisprod/app/.env

  # -- Build image locally (bootstrap: GHCR may not have the image yet) ---------
  - cd /opt/localisprod/app && docker build -t ghcr.io/gsarmaonline/localisprod-v2:latest .

  # -- Start services -----------------------------------------------------------
  - cd /opt/localisprod/app && docker compose up -d
