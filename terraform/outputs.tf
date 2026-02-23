output "droplet_ip" {
  description = "Public IPv4 address of the localisprod droplet"
  value       = digitalocean_droplet.localisprod.ipv4_address
}

output "app_url" {
  description = "URL to access the localisprod web UI"
  value       = "https://${var.domain}"
}

output "ssh_command" {
  description = "SSH command to connect to the droplet"
  value       = "ssh root@${digitalocean_droplet.localisprod.ipv4_address}"
}
