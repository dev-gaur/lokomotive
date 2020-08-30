// Copyright 2020 The Lokomotive Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package velero //nolint:testpackage

import (
	"testing"

	"github.com/hashicorp/hcl/v2"

	"github.com/kinvolk/lokomotive/pkg/components/util"
)

func TestEmptyConfig(t *testing.T) {
	c := newComponent()
	emptyConfig := hcl.EmptyBody()
	evalContext := hcl.EvalContext{}

	diagnostics := c.LoadConfig(&emptyConfig, &evalContext)
	if !diagnostics.HasErrors() {
		t.Errorf("Empty config should return error")
	}
}

func TestRenderValidManifests(t *testing.T) { //nolint:funlen
	configHCLAzure := `
component "velero" {
  provider = "azure"
  azure {
    subscription_id  = "foo"
    tenant_id        = "foo"
    client_id        = "foo"
    client_secret    = "foo"
    resource_group   = "foo"

    backup_storage_location {
      resource_group  = "foo"
      storage_account = "foo"
      bucket          = "foo"
    }
  }
}
`
	configHCLOpenEBS := `
component "velero" {
  provider = "openebs"
  openebs {
    credentials  = <<EOF
[default]
access_key = "access_key"
secret_key = "secret_key"
EOF

    volume_snapshot_location {
      bucket           = "test_bucket"
      provider         = "aws"
      region           = "eu-west-1"
    }

    backup_storage_location {
      bucket           = "test_bucket"
      provider         = "aws"
      region           = "eu-west-1"
    }
  }
}
`
	configHCLRestic := `
component "velero" {
  provider = "restic"
  restic {
    credentials  = <<EOF
[default]
access_key = "access_key"
secret_key = "secret_key"
EOF

    backup_storage_location {
      bucket           = "test_bucket"
      provider         = "aws"
    }
  }
}
`

	configHCLs := []string{configHCLAzure, configHCLOpenEBS, configHCLRestic}
	for _, configHCL := range configHCLs {
		component := newComponent()

		body, diagnostics := util.GetComponentBody(configHCL, name)
		if diagnostics != nil {
			t.Fatalf("Error getting component body: %v", diagnostics)
		}

		diagnostics = component.LoadConfig(body, &hcl.EvalContext{})
		if diagnostics.HasErrors() {
			t.Fatalf("Valid config for provider %q should not return error, got: %s",
				component.Provider, diagnostics)
		}

		m, err := component.RenderManifests()
		if err != nil {
			t.Fatalf("Rendering manifests with valid config for provider %q should succeed,got: %s",
				component.Provider, err)
		}

		if len(m) == 0 {
			t.Fatalf("Rendered manifests shouldn't be empty")
		}
	}
}
