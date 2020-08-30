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

package velero

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"

	"github.com/kinvolk/lokomotive/internal/template"
	"github.com/kinvolk/lokomotive/pkg/components"
	"github.com/kinvolk/lokomotive/pkg/components/util"
	"github.com/kinvolk/lokomotive/pkg/components/velero/azure"
	"github.com/kinvolk/lokomotive/pkg/components/velero/openebs"
	"github.com/kinvolk/lokomotive/pkg/k8sutil"
)

const name = "velero"

// init registers velero component to components list, so it shows up as available to install
func init() {
	components.Register(name, newComponent())
}

// component represents component configuration data
type component struct {
	Provider string `hcl:"provider"`
	// Namespace where velero resources should be installed. Defaults to 'velero'.
	Namespace string `hcl:"namespace,optional"`
	// Metrics specific configuration
	Metrics *Metrics `hcl:"metrics,block"`

	// Azure specific parameters
	Azure *azure.Configuration `hcl:"azure,block"`
	// OpenEBS specific parameters.
	OpenEBS *openebs.Configuration `hcl:"openebs,block"`
}

// Metrics represents prometheus specific parameters
type Metrics struct {
	Enabled        bool `hcl:"enabled,optional"`
	ServiceMonitor bool `hcl:"service_monitor,optional"`
}

// Provider requires implementing config validation function for each provider
type provider interface {
	IndentCredentials()
	ChartValuesTemplate() string
	Validate() hcl.Diagnostics
}

// newComponent creates new velero component struct with default values initialized
func newComponent() *component {
	return &component{
		Namespace: "velero",
		Metrics: &Metrics{
			Enabled:        false,
			ServiceMonitor: false,
		},
		OpenEBS: openebs.NewConfiguration(),
	}
}

// LoadConfig decodes given HCL and validates the configuration.
//
// If it finds any problems, HCL diagnostics array is returned containing error messages.
func (c *component) LoadConfig(configBody *hcl.Body, evalContext *hcl.EvalContext) hcl.Diagnostics {
	diagnostics := hcl.Diagnostics{}

	// If config is not defined at all, replace it with just empty struct, so we can
	// deserialize it and proceed
	if configBody == nil {
		// Perhaps we can skip this error?
		diagnostics = append(diagnostics, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "component requires configuration",
			Detail:   "component has required fields in it's configuration, so configuration block must be created",
		})
		emptyConfig := hcl.EmptyBody()
		configBody = &emptyConfig
	}

	if err := gohcl.DecodeBody(*configBody, evalContext, c); err != nil {
		diagnostics = append(diagnostics, err...)
	}

	// Validate component's configuration
	diagnostics = append(diagnostics, c.validate()...)

	if diagnostics.HasErrors() {
		return diagnostics
	}

	return nil
}

// RenderManifest read helm chart from assets and renders it into list of files
func (c *component) RenderManifests() (map[string]string, error) {
	helmChart, err := components.Chart(name)
	if err != nil {
		return nil, fmt.Errorf("retrieving chart from assets: %w", err)
	}

	p, err := c.getProvider()
	if err != nil {
		return nil, err
	}
	// Indent credentials
	p.IndentCredentials()

	values, err := template.Render(p.ChartValuesTemplate(), c)
	if err != nil {
		return nil, fmt.Errorf("rendering chart values template: %w", err)
	}

	renderedFiles, err := util.RenderChart(helmChart, name, c.Namespace, values)
	if err != nil {
		return nil, fmt.Errorf("rendering chart: %w", err)
	}

	return renderedFiles, nil
}

// validate validates component configuration
func (c *component) validate() hcl.Diagnostics {
	diagnostics := hcl.Diagnostics{}
	// Supported providers.
	supportedProviders := c.getSupportedProviders()

	// Select provider and validate it's configuration
	p, err := c.getProvider()
	if err != nil {
		return append(diagnostics, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf("provider must be one of: '%s'", strings.Join(supportedProviders[:], "', '")),
			Detail:   "Make sure to set provider to one of supported values",
		})
	}

	return append(diagnostics, p.Validate()...)
}

// getSupportedProviders returns a list of supported providers.
func (c *component) getSupportedProviders() []string {
	return []string{"azure", "openebs"}
}

// getProvider returns correct provider interface based on component configuration
func (c *component) getProvider() (provider, error) {
	switch c.Provider {
	case "azure":
		return c.Azure, nil
	case "openebs":
		return c.OpenEBS, nil
	default:
		return nil, fmt.Errorf("unsupported provider '%s'", c.Provider)
	}
}

func (c *component) Metadata() components.Metadata {
	return components.Metadata{
		Name: name,
		Namespace: k8sutil.Namespace{
			Name: c.Namespace,
		},
	}
}
