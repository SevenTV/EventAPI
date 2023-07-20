data "terraform_remote_state" "infra" {
  backend = "remote"

  config = {
    organization = "7tv"
    workspaces = {
      name = local.infra_workspace_name
    }
  }
}

variable "region" {
  description = "AWS region"
  type        = string
  default     = "us-east-2"
}

variable "namespace" {
  type    = string
  default = "eventapi"
}

variable "image_url" {
  type = string
}

variable "heartbeat_interval" {
  type    = number
  default = 28000
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
