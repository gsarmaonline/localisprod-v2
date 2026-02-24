#cloud-config

package_update: true
package_upgrade: true
packages:
  - curl

runcmd:
  # -- Swap (essential for 1 GB droplets: prevents OOM during Docker pulls) --
  - fallocate -l 2G /swapfile
  - chmod 600 /swapfile
  - mkswap /swapfile
  - swapon /swapfile
  - echo '/swapfile none swap sw 0 0' >> /etc/fstab

  # -- Docker ----------------------------------------------------------------
  - curl -fsSL https://get.docker.com | sh
  - systemctl enable docker
  - systemctl start docker

  # -- App directory ---------------------------------------------------------
  - mkdir -p /opt/localisprod/app

  # -- docker-compose.yml (pulled from repo, kept in sync by cron) ----------
  - curl -fsSL https://raw.githubusercontent.com/gsarmaonline/localisprod-v2/main/docker-compose.yml -o /opt/localisprod/app/docker-compose.yml

  # -- Write .env ------------------------------------------------------------
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

  # -- Start services --------------------------------------------------------
  - docker compose -f /opt/localisprod/app/docker-compose.yml pull
  - docker compose -f /opt/localisprod/app/docker-compose.yml up -d

  # -- Compose sync (curl latest docker-compose.yml from GitHub every 5 min) -
  - |
    cat > /opt/localisprod/sync.sh <<'SYNCEOF'
    #!/bin/bash
    set -euo pipefail
    curl -fsSL https://raw.githubusercontent.com/gsarmaonline/localisprod-v2/main/docker-compose.yml \
      -o /opt/localisprod/app/docker-compose.yml
    docker compose -f /opt/localisprod/app/docker-compose.yml up -d --remove-orphans
    SYNCEOF
  - chmod +x /opt/localisprod/sync.sh
  - echo '*/5 * * * * root /opt/localisprod/sync.sh >> /var/log/localisprod-sync.log 2>&1' > /etc/cron.d/localisprod-sync
