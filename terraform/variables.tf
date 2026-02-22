variable "do_token" {
  description = "DigitalOcean API token"
  type        = string
  sensitive   = true
}

variable "ssh_key_fingerprint" {
  description = "Fingerprint of an SSH key already added to your DigitalOcean account"
  type        = string
}

variable "region" {
  description = "DigitalOcean region slug"
  type        = string
  default     = "nyc3"
}

variable "droplet_size" {
  description = "Droplet size slug"
  type        = string
  default     = "s-1vcpu-1gb"
}

variable "repo_url" {
  description = "Git clone URL for the localisprod repo. For private repos use SSH format: git@github.com:org/repo.git (requires github_deploy_key). For public repos HTTPS works: https://github.com/org/repo.git"
  type        = string
}

variable "github_deploy_key" {
  description = "Private SSH key granted read access to the repo (GitHub deploy key). Required for private repos. Leave empty for public repos."
  type        = string
  sensitive   = true
  default     = ""
}

variable "secret_key" {
  description = "Base64-encoded 32-byte key for AES-256-GCM encryption of env vars. Generate with: openssl rand -base64 32"
  type        = string
  sensitive   = true
  default     = ""
}

variable "port" {
  description = "HTTP port the server listens on"
  type        = number
  default     = 8080
}
