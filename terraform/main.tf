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
  infra                = data.terraform_remote_state.infra.outputs
  image_url = var.image_url != null ? var.image_url : format("ghcr.io/seventv/eventapi:%s-latest", trimprefix(terraform.workspace, "seventv-eventapi-"))
}
