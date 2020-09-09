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

package headlamp

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"

	"github.com/kinvolk/lokomotive/internal/template"
	"github.com/kinvolk/lokomotive/pkg/components"
	"github.com/kinvolk/lokomotive/pkg/components/util"
	"github.com/kinvolk/lokomotive/pkg/k8sutil"
)

const name = "headlamp"

const chartValuesTmpl = `
nodeSelector:
  beta.kubernetes.io/os: linux
ingress:
  enabled: true
  hosts:
  - host: {{ .IngressHost }}
    paths:
    - /
  tls:
  - secretName: {{ .IngressHost }}-tls
    hosts:
    - {{ .IngressHost }}
  annotations:
    cert-manager.io/cluster-issuer: {{ .CertManagerClusterIssuer }}
    contour.heptio.com/websocket-routes: "/"
`

//nolint:gochecknoinits
func init() {
	components.Register(name, newComponent())
}

type component struct {
	IngressHost              string `hcl:"ingress_host,attr"`
	CertManagerClusterIssuer string `hcl:"certmanager_cluster_issuer,optional"`
}

func newComponent() *component {
	return &component{
		CertManagerClusterIssuer: "letsencrypt-production",
	}
}

// LoadConfig loads the component config.
func (c *component) LoadConfig(configBody *hcl.Body, evalContext *hcl.EvalContext) hcl.Diagnostics {
	if configBody == nil {
		return hcl.Diagnostics{
			components.HCLDiagConfigBodyNil,
		}
	}

	return gohcl.DecodeBody(*configBody, evalContext, c)
}

// RenderManifests renders the Helm chart templates with values provided.
func (c *component) RenderManifests() (map[string]string, error) {
	helmChart, err := components.Chart(name)
	if err != nil {
		return nil, fmt.Errorf("retrieving chart from assets: %w", err)
	}

	values, err := template.Render(chartValuesTmpl, c)
	if err != nil {
		return nil, fmt.Errorf("rendering chart values template failed: %w", err)
	}

	renderedFiles, err := util.RenderChart(helmChart, name, c.Metadata().Namespace.Name, values)
	if err != nil {
		return nil, fmt.Errorf("rendering chart failed: %w", err)
	}

	return renderedFiles, nil
}

func (c *component) Metadata() components.Metadata {
	return components.Metadata{
		Name: name,
		Namespace: k8sutil.Namespace{
			Name: "kube-system",
		},
	}
}
