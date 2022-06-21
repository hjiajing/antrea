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
	"k8s.io/client-go/rest"
	"path"

	"sigs.k8s.io/controller-runtime/pkg/client"

	multiclusterscheme "antrea.io/antrea/pkg/antctl/raw/multicluster/scheme"
	"antrea.io/antrea/pkg/antctl/runtime"
)

type deployer struct {
	antreaVersion     string
	leaderServer      string
	kubeconfigDir     string
	leaderCluster     string
	memberClusters    []string
	leaderNamespace   string
	memberNamespace   string
	leaderKubeConfig  *rest.Config
	memberKubeConfigs map[string]*rest.Config

	leaderClient client.Client
	memberClient map[string]client.Client
}

func newDeployer(options *initOptions) (*deployer, error) {
	leaderClusterName, memberClusterNames, err := options.parseKubeconfigs()
	if err != nil {
		return nil, err
	}

	leaderConfig, err := runtime.ResolveKubeconfig(path.Join(options.kubeconfigDir, leaderClusterName))
	if err != nil {
		return nil, err
	}
	leaderClient, err := client.New(leaderConfig, client.Options{Scheme: multiclusterscheme.Scheme})
	if err != nil {
		return nil, err
	}
	leaderServer := leaderConfig.Host

	memberClients := map[string]client.Client{}
	memberKubeConfigs := map[string]*rest.Config{}
	for _, memberClusterName := range memberClusterNames {
		memberClusterConfig, err := runtime.ResolveKubeconfig(path.Join(options.kubeconfigDir, memberClusterName))
		if err != nil {
			return nil, err
		}
		memberKubeConfigs[memberClusterName] = memberClusterConfig

		memberClient, err := client.New(memberClusterConfig, client.Options{Scheme: multiclusterscheme.Scheme})
		if err != nil {
			return nil, err
		}

		memberClients[memberClusterName] = memberClient
	}

	return &deployer{
		leaderKubeConfig:  leaderConfig,
		memberKubeConfigs: memberKubeConfigs,
		antreaVersion:     options.antreaVersion,
		leaderServer:      leaderServer,
		kubeconfigDir:     options.kubeconfigDir,
		leaderCluster:     leaderClusterName,
		memberClusters:    memberClusterNames,
		leaderNamespace:   options.leaderNamespace,
		memberNamespace:   options.memberNamespace,
		leaderClient:      leaderClient,
		memberClient:      memberClients,
	}, nil
}

type deploy interface {
}
