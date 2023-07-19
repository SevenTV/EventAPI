terraform {
  backend "remote" {
    hostname     = "app.terraform.io"
    organization = "7tv"

    workspaces {
      prefix = "seventv-eventapi-"
    }
  }
}

locals {
  infra_workspace_name = replace(terraform.workspace, "eventapi", "infra")
  infra = data.terraform_remote_state.infra.outputs
}
