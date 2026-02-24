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

variable "secret_key" {
  description = "Base64-encoded 32-byte key for AES-256-GCM encryption of env vars. Generate with: openssl rand -base64 32"
  type        = string
  sensitive   = true
  default     = ""
}

variable "jwt_secret" {
  description = "Secret key for signing JWTs. Generate with: openssl rand -base64 32"
  type        = string
  sensitive   = true
}

variable "google_client_id" {
  description = "Google OAuth client ID"
  type        = string
  sensitive   = true
}

variable "google_client_secret" {
  description = "Google OAuth client secret"
  type        = string
  sensitive   = true
}

variable "domain" {
  description = "Domain name to serve the app on (must point to the droplet IP in DNS)"
  type        = string
  default     = "localisprod.com"
}

variable "acme_email" {
  description = "Email address for Let's Encrypt certificate notifications"
  type        = string
}

