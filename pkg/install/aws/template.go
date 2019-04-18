package aws

var terraformConfigTmpl = `
module "aws-{{.ClusterName}}" {
  source = "{{.Source}}"

  providers = {
    aws      = "aws.default"
    local    = "local.default"
    null     = "null.default"
    template = "template.default"
    tls      = "tls.default"
  }

  cluster_name = "{{.ClusterName}}"
  dns_zone     = "{{.DNSZone}}"
  dns_zone_id  = "{{.DNSZoneID}}"

  ssh_authorized_key = "{{.SSHAuthorizedKey}}"
  asset_dir          = "{{.AssetDir}}"

  worker_count = 2
  worker_type  = "t3.small"

  os_image = "{{.OSImage}}"
}

provider "aws" {
  version = "~> 1.13.0"
  alias   = "default"

  region                  = "eu-central-1"
  shared_credentials_file = "{{.CredsPath}}"
}

provider "local" {
  version = "~> 1.0"
  alias   = "default"
}

provider "null" {
  version = "~> 1.0"
  alias   = "default"
}

provider "template" {
  version = "~> 1.0"
  alias   = "default"
}

provider "tls" {
  version = "~> 1.0"
  alias   = "default"
}
`
