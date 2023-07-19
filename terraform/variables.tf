resource "terraform_remote_state" "infra" {
    backend = "remote"

    config = {
        organization = "7tv"
        workspaces = {
            name = "seventv-infra-${trimprefix(terraform.workspace, "eventapi")}"
        }
    }
}

variable "region" {
  description = "AWS region"
  type        = string
  default     = "us-east-2"
}

variable "cloudflare_api_token" {
  description = "Cloudflare API Token"
  sensitive   = true
}

variable "namespace" {
  type    = string
  default = "eventapi"
}

variable "app_docker_image" {
  type = string
  default = "ghcr.io/seventv/eventapi:latest"
}

variable "heartbeat_interval" {
  type    = number
  default = 28
}

variable "subscription_limit" {
  type    = number
  default = 500
}

variable "connection_limit" {
  type    = number
  default = 10000
}

variable "ttl" {
  type    = number
  default = 60
}
