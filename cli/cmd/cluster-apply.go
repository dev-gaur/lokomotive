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

package cmd

import (
	"fmt"

	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/kinvolk/lokomotive/internal"
	"github.com/kinvolk/lokomotive/pkg/helm"
	"github.com/kinvolk/lokomotive/pkg/install"
	"github.com/kinvolk/lokomotive/pkg/k8sutil"
	"github.com/kinvolk/lokomotive/pkg/lokomotive"
	"github.com/kinvolk/lokomotive/pkg/platform"
)

var (
	verbose         bool
	skipComponents  bool
	upgradeKubelets bool
)

var clusterApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Deploy or update a cluster",
	Long: `Deploy or update a cluster.
Deploys a cluster if it isn't deployed, otherwise updates it.
Unless explicitly skipped, components listed in the configuration are applied as well.`,
	Run: runClusterApply,
}

func init() {
	clusterCmd.AddCommand(clusterApplyCmd)
	pf := clusterApplyCmd.PersistentFlags()
	pf.BoolVarP(&confirm, "confirm", "", false, "Upgrade cluster without asking for confirmation")
	pf.BoolVarP(&verbose, "verbose", "v", false, "Show output from Terraform")
	pf.BoolVarP(&skipComponents, "skip-components", "", false, "Skip applying component configuration")
	pf.BoolVarP(&upgradeKubelets, "upgrade-kubelets", "", false, "Experimentally upgrade self-hosted kubelets")
}

//nolint:funlen,gocognit
func runClusterApply(cmd *cobra.Command, args []string) {
	ctxLogger := log.WithFields(log.Fields{
		"command": "lokoctl cluster apply",
		"args":    args,
	})

	cc, c, ex := initialize(ctxLogger)

	assetDir, err := homedir.Expand(c.AssetDir())
	if err != nil {
		ctxLogger.Fatalf("Error expanding path: %v", err)
	}

	exists := clusterExists(ctxLogger, ex)
	if exists && !confirm {
		// TODO: We could plan to a file and use it when installing.
		// TODO: How does this play with a complex execution plan? Does a single "global" plan
		// operation represent what's going to be applied?
		if err := ex.Plan(); err != nil {
			ctxLogger.Fatalf("Failed to reconcile cluster state: %v", err)
		}

		if !askForConfirmation("Do you want to proceed with cluster apply?") {
			ctxLogger.Println("Cluster apply cancelled")

			return
		}
	}

	for _, step := range c.TerraformExecutionPlan() {
		if step.PreExecutionHook != nil {
			ctxLogger.Printf("Running pre-execution hook for step %q", step.Description)

			if err := step.PreExecutionHook(ex); err != nil {
				ctxLogger.Fatalf("Pre-execution hook for step %q failed: %v", step.Description, err)
			}
		}

		err := ex.Execute(step)
		if err != nil {
			ctxLogger.Fatalf("Execution of step %q failed: %v", step.Description, err)
		}
	}

	fmt.Printf("\nYour configurations are stored in %s\n", assetDir)

	kubeconfig, err := getKubeconfig(ctxLogger, cc, true)
	if err != nil {
		ctxLogger.Fatalf("Failed to get kubeconfig: %v", err)
	}

	if err := verifyCluster(kubeconfig, c.Nodes()); err != nil {
		ctxLogger.Fatalf("Verify cluster: %v", err)
	}

	// Update all the pre installed namespaces with lokomotive specific label.
	// `lokomotive.kinvolk.io/name: <namespace_name>`.
	if err := updateInstalledNamespaces(kubeconfig); err != nil {
		ctxLogger.Fatalf("Updating installed namespace: %v", err)
	}

	// Do controlplane upgrades only if cluster already exists and it is not a managed platform.
	if exists && !c.Managed() {
		fmt.Printf("\nEnsuring that cluster controlplane is up to date.\n")

		cu := controlplaneUpdater{
			kubeconfig: kubeconfig,
			assetDir:   assetDir,
			ctxLogger:  *ctxLogger,
			ex:         *ex,
		}

		charts := platform.CommonControlPlaneCharts()

		if upgradeKubelets {
			charts = append(charts, helm.LokomotiveChart{
				Name:      "kubelet",
				Namespace: "kube-system",
			})
		}

		for _, c := range charts {
			cu.upgradeComponent(c.Name, c.Namespace)
		}
	}

	hooks := c.PostApplyHooks()
	if len(hooks) > 0 {
		for _, h := range hooks {
			ctxLogger.Infof("Running post apply hook %q", h.Description)

			if err := h.Function(kubeconfig); err != nil {
				ctxLogger.Fatalf("Post apply hook %q failed: %v", h.Description, err)
			}
		}
	}

	if skipComponents {
		return
	}

	componentsToApply := []string{}
	for _, component := range cc.RootConfig.Components {
		componentsToApply = append(componentsToApply, component.Name)
	}

	ctxLogger.Println("Applying component configuration")

	if len(componentsToApply) > 0 {
		if err := applyComponents(cc, kubeconfig, componentsToApply...); err != nil {
			ctxLogger.Fatalf("Applying component configuration failed: %v", err)
		}
	}
}

func verifyCluster(kubeconfig []byte, expectedNodes int) error {
	cs, err := k8sutil.NewClientset(kubeconfig)
	if err != nil {
		return fmt.Errorf("creating Kubernetes clientset: %w", err)
	}

	cluster := lokomotive.NewCluster(cs, expectedNodes)

	return install.Verify(cluster)
}

func updateInstalledNamespaces(kubeconfig []byte) error {
	cs, err := k8sutil.NewClientset(kubeconfig)
	if err != nil {
		return fmt.Errorf("create clientset: %v", err)
	}

	nsclient := cs.CoreV1().Namespaces()

	namespaces, err := k8sutil.ListNamespaces(nsclient)
	if err != nil {
		return fmt.Errorf("getting list of namespaces: %v", err)
	}

	for _, ns := range namespaces.Items {
		ns := k8sutil.Namespace{
			Name: ns.ObjectMeta.Name,
			Labels: map[string]string{
				internal.NamespaceLabelKey: ns.ObjectMeta.Name,
			},
		}

		if err := k8sutil.CreateOrUpdateNamespace(ns, nsclient); err != nil {
			return fmt.Errorf("namespace %q with labels: %v", ns, err)
		}
	}

	return nil
}
