// Copyright 2022 Antrea Authors
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

package init

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/spf13/cobra"
)

type initOptions struct {
	kubeconfigDir   string
	leaderCluster   string
	leaderNamespace string
	memberNamespace string
	antreaVersion   string
}

var initOpts *initOptions

var initExamples = strings.Trim(`
# Initialize the Antrea MultiCluster in a specified Namespace. The "antrea-mc-controller" Deployment will be deployed 
and the CRDs will be defined.
  $ antctl mc init --antrea-version <ANTREA_VERSION> -n <NAMESPACE> --kubeconfig-dir <KUBECONFIG_DIR> --leader-cluster <LEADER_CLUSTER_CONFIG>

The following CRDs will be defined:
- CRDs: ClusterClaim, ClusterSet, MemberClusterAnnounce, ResourceExport, ResourceImport, ServiceExport, ServiceImport, Gateway, ClusterInfo
`, "\n")

func (o *initOptions) validateAndComplete() error {
	if o.kubeconfigDir == "" {
		return fmt.Errorf("the kubeconfig Dir cannot be empty")
	}
	if o.leaderCluster == "" {
		return fmt.Errorf("the leader cluster must be specified")
	}
	if o.leaderNamespace == "" {
		return fmt.Errorf("the leader cluster Namespace cannot be empty")
	}
	if o.memberNamespace == "" {
		return fmt.Errorf("the member cluster Namespace cannot be empty")
	}
	if o.antreaVersion == "" {
		o.antreaVersion = "latest"
	}

	return nil
}

func NewInitCmd() *cobra.Command {
	command := &cobra.Command{
		Use:     "init",
		Args:    cobra.MaximumNArgs(0),
		Short:   "Initialize the Antrea Multi-cluster",
		Example: initExamples,
		RunE:    initRunE,
	}

	o := &initOptions{}
	initOpts = o
	command.Flags().StringVarP(&o.leaderNamespace, "leader-namespace", "", "antrea-multicluster", "Namespace to deploy Antrea Multi-cluster in leader cluster")
	command.Flags().StringVarP(&o.memberNamespace, "member-namespace", "", "kube-system", "Namespace to deploy Antrea Multi-cluster in member cluster")
	command.Flags().StringVarP(&o.leaderCluster, "leader-cluster", "", "", "the leader cluster of the Antrea Multi-cluster")
	command.Flags().StringVarP(&o.kubeconfigDir, "kubeconfig-dir", "", defaultKubeconfigPath(), "directory of the kubeconfigs")
	command.Flags().StringVarP(&o.antreaVersion, "antrea-version", "", "latest", "version of the Antrea Multi-cluster. If not set, the latest version from Antrea main branch will be used")

	return command
}

func initRunE(cmd *cobra.Command, _ []string) error {
	if err := initOpts.validateAndComplete(); err != nil {
		return err
	}

	antreaMulticlusterDeployer, err := newDeployer(initOpts)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Deploying Antrea Multi-cluster to the leader cluster %s\n", antreaMulticlusterDeployer.leaderCluster)
	if err := antreaMulticlusterDeployer.deployController(cmd, leaderRole, antreaMulticlusterDeployer.leaderCluster); err != nil {
		return err
	}
	for _, memberCluster := range antreaMulticlusterDeployer.memberClusters {
		fmt.Fprintf(cmd.OutOrStdout(), "Deploying Antrea Multi-cluster to member cluster %s\n", memberCluster)
		if err := antreaMulticlusterDeployer.deployController(cmd, memberRole, memberCluster); err != nil {
			return err
		}
	}
	if err := antreaMulticlusterDeployer.createAccessToken(cmd); err != nil {
		return err
	}
	for _, memberCluster := range antreaMulticlusterDeployer.memberClusters {
		if err := antreaMulticlusterDeployer.deployClusterSet(cmd, memberRole, memberCluster); err != nil {
			return err
		}
	}
	if err := antreaMulticlusterDeployer.deployClusterSet(cmd, leaderRole, antreaMulticlusterDeployer.leaderCluster); err != nil {
		return err
	}

	return nil
}

func defaultKubeconfigPath() string {
	homeDir, _ := os.UserHomeDir()
	return path.Join(homeDir, ".kube")
}

func (o *initOptions) parseKubeconfigs() (string, []string, error) {
	var leaderCluster string
	var memberClusters []string
	var err error

	kubeconfigs, err := ioutil.ReadDir(o.kubeconfigDir)
	if err != nil {
		return leaderCluster, memberClusters, err
	}
	for _, kubeconfig := range kubeconfigs {
		if kubeconfig.IsDir() {
			continue
		}
		if kubeconfig.Name() == o.leaderCluster {
			leaderCluster = kubeconfig.Name()
		} else {
			memberClusters = append(memberClusters, kubeconfig.Name())
		}
	}

	if leaderCluster == "" {
		err = fmt.Errorf("the kubeconfig of leader cluster does not exist")
	}
	if len(memberClusters) < 2 {
		err = fmt.Errorf("there must be 2 member cluster at least")
	}

	return leaderCluster, memberClusters, err
}
