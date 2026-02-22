terraform {
  required_providers {
    digitalocean = {
      source  = "digitalocean/digitalocean"
      version = "~> 2.0"
    }
  }
  required_version = ">= 1.3"
}

provider "digitalocean" {
  token = var.do_token
}

resource "digitalocean_droplet" "localisprod" {
  name      = "localisprod"
  region    = var.region
  size      = var.droplet_size
  image     = "ubuntu-22-04-x64"
  ssh_keys  = [var.ssh_key_fingerprint]
  user_data = templatefile("${path.module}/cloud-init.yaml.tpl", {
    repo_url          = var.repo_url
    secret_key        = var.secret_key
    port              = var.port
    github_deploy_key = var.github_deploy_key
  })
}

resource "digitalocean_firewall" "localisprod" {
  name        = "localisprod-fw"
  droplet_ids = [digitalocean_droplet.localisprod.id]

  inbound_rule {
    protocol         = "tcp"
    port_range       = "22"
    source_addresses = ["0.0.0.0/0", "::/0"]
  }

  inbound_rule {
    protocol         = "tcp"
    port_range       = tostring(var.port)
    source_addresses = ["0.0.0.0/0", "::/0"]
  }

  inbound_rule {
    protocol         = "icmp"
    source_addresses = ["0.0.0.0/0", "::/0"]
  }

  outbound_rule {
    protocol              = "tcp"
    port_range            = "1-65535"
    destination_addresses = ["0.0.0.0/0", "::/0"]
  }

  outbound_rule {
    protocol              = "udp"
    port_range            = "1-65535"
    destination_addresses = ["0.0.0.0/0", "::/0"]
  }

  outbound_rule {
    protocol              = "icmp"
    destination_addresses = ["0.0.0.0/0", "::/0"]
  }
}
