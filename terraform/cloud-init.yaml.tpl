#cloud-config

package_update: true
package_upgrade: true
packages:
  - git
  - curl
  - wget
  - make

write_files:
  - path: /etc/systemd/system/localisprod.service
    content: |
      [Unit]
      Description=Localisprod Cluster Manager
      After=network.target

      [Service]
      Type=simple
      User=localisprod
      WorkingDirectory=/opt/localisprod/app
      EnvironmentFile=/opt/localisprod/app/.env
      ExecStart=/opt/localisprod/app/bin/server
      Restart=on-failure
      RestartSec=5
      StandardOutput=journal
      StandardError=journal

      [Install]
      WantedBy=multi-user.target

runcmd:
  # ── Swap (essential for 1 GB droplets: npm build can easily OOM) ─────────
  - fallocate -l 1G /swapfile
  - chmod 600 /swapfile
  - mkswap /swapfile
  - swapon /swapfile
  - echo '/swapfile none swap sw 0 0' >> /etc/fstab

  # ── App user ─────────────────────────────────────────────────────────────
  - useradd -r -s /bin/false -m -d /opt/localisprod localisprod

  # ── Go 1.24 ──────────────────────────────────────────────────────────────
  - wget -q https://go.dev/dl/go1.24.1.linux-amd64.tar.gz -O /tmp/go.tar.gz
  - tar -C /usr/local -xzf /tmp/go.tar.gz
  - rm /tmp/go.tar.gz
  - echo 'export PATH=$PATH:/usr/local/go/bin' > /etc/profile.d/go.sh

  # ── Node.js 20 LTS ───────────────────────────────────────────────────────
  - curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
  - apt-get install -y nodejs

  # ── GitHub deploy key (needed for private repos) ─────────────────────────
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

  # ── Clone repo ───────────────────────────────────────────────────────────
  - git clone ${repo_url} /opt/localisprod/app

  # ── Write .env (after git clone creates the directory) ───────────────────
  - |
    cat > /opt/localisprod/app/.env <<'ENVEOF'
    PORT=${port}
    DB_PATH=/opt/localisprod/app/cluster.db
    SECRET_KEY=${secret_key}
    ENVEOF
  - chmod 600 /opt/localisprod/app/.env

  # ── Build frontend ────────────────────────────────────────────────────────
  - cd /opt/localisprod/app/web && npm ci && npm run build

  # ── Build backend ─────────────────────────────────────────────────────────
  # Must run from the module root so go.mod is found; bin/ is gitignored so mkdir first
  - mkdir -p /opt/localisprod/app/bin
  - cd /opt/localisprod/app && /usr/local/go/bin/go build -o bin/server ./cmd/server/main.go

  # ── Fix ownership ─────────────────────────────────────────────────────────
  - chown -R localisprod:localisprod /opt/localisprod

  # ── Start service ─────────────────────────────────────────────────────────
  - systemctl daemon-reload
  - systemctl enable localisprod
  - systemctl start localisprod
