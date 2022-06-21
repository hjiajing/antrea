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
	"bytes"
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"io/ioutil"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/util/wait"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"net/http"
	"strings"
	"time"
)

const (
	leaderRole = "leader"
	memberRole = "member"

	latestVersionURL     = "https://raw.githubusercontent.com/antrea-io/antrea/main/multicluster/build/yamls"
	downloadURL          = "https://github.com/antrea-io/antrea/releases/download"
	leaderGlobalYAML     = "antrea-multicluster-leader-global.yml"
	leaderNamespacedYAML = "antrea-multicluster-leader-namespaced.yml"
	memberYAML           = "antrea-multicluster-member.yml"
)

func generateManifest(role string, version string) ([]string, error) {
	var manifests []string
	switch role {
	case leaderRole:
		manifests = []string{
			fmt.Sprintf("%s/%s", latestVersionURL, leaderGlobalYAML),
			fmt.Sprintf("%s/%s", latestVersionURL, leaderNamespacedYAML),
		}
		if version != "latest" {
			manifests = []string{
				fmt.Sprintf("%s/%s/%s", downloadURL, version, leaderGlobalYAML),
				fmt.Sprintf("%s/%s/%s", downloadURL, version, leaderNamespacedYAML),
			}
		}
	case memberRole:
		manifests = []string{
			fmt.Sprintf("%s/%s", latestVersionURL, memberYAML),
		}
		if version != "latest" {
			manifests = []string{
				fmt.Sprintf("%s/%s/%s", downloadURL, version, memberYAML),
			}
		}
	default:
		return manifests, fmt.Errorf("invalid role %s", role)
	}

	return manifests, nil
}

func (d *deployer) deployController(cmd *cobra.Command, role string, clusterName string) error {
	manifests, err := generateManifest(role, d.antreaVersion)
	if err != nil {
		return err
	}

	for _, manifest := range manifests {
		// #nosec G107
		resp, err := http.Get(manifest)
		if err != nil {
			return err
		}
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		content := string(b)
		if role == leaderRole && strings.Contains(manifest, "namespaced") {
			content = strings.ReplaceAll(content, "antrea-multicluster", d.leaderNamespace)
		}
		if role == memberRole && strings.Contains(manifest, "member") {
			content = strings.ReplaceAll(content, "kube-system", d.memberNamespace)
		}
		if err := d.createResources(cmd, []byte(content), clusterName); err != nil {
			return err
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "waiting for Antrea Multicluster Deployment rollout\n")
	var kubeconfig *rest.Config
	var namespace string
	if clusterName == d.leaderCluster {
		kubeconfig = d.leaderKubeConfig
		namespace = d.leaderNamespace
	} else {
		kubeconfig = d.memberKubeConfigs[clusterName]
		namespace = d.memberNamespace
	}
	k8sClient, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return err
	}
	mcController := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "antrea-mc-controller",
			Namespace: namespace,
		},
	}

	if err := waitForDeploymentRollout(k8sClient, mcController); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Antrea Multicluster controller is running\n")
	fmt.Fprintf(cmd.OutOrStdout(), "Antrea Multi-cluster Controller deployed in cluster %s\n", clusterName)

	return nil
}

func (d *deployer) createResources(cmd *cobra.Command, content []byte, clusterName string) error {
	var kubeconfig *rest.Config
	if clusterName == d.leaderCluster {
		kubeconfig = d.leaderKubeConfig
	} else {
		kubeconfig = d.memberKubeConfigs[clusterName]
	}
	k8sClient, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return err
	}
	dynamicClient, err := dynamic.NewForConfig(kubeconfig)
	if err != nil {
		return err
	}

	decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(content)), 100)
	for {
		var rawObj runtime.RawExtension
		if err = decoder.Decode(&rawObj); err != nil {
			break
		}

		obj, gvk, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
		if err != nil {
			return err
		}
		unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			return err
		}

		unstructuredObj := &unstructured.Unstructured{Object: unstructuredMap}

		gr, err := restmapper.GetAPIGroupResources(k8sClient.Discovery())
		if err != nil {
			return err
		}

		mapper := restmapper.NewDiscoveryRESTMapper(gr)
		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return err
		}

		var dri dynamic.ResourceInterface
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			dri = dynamicClient.Resource(mapping.Resource).Namespace(unstructuredObj.GetNamespace())
		} else {
			dri = dynamicClient.Resource(mapping.Resource)
		}

		if _, err := dri.Create(context.TODO(), unstructuredObj, metav1.CreateOptions{}); err != nil {
			if !kerrors.IsAlreadyExists(err) {
				return err
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s/%s created\n", unstructuredObj.GetKind(), unstructuredObj.GetName())
	}

	return nil
}

func waitForDeploymentRollout(kubeClientSet kubernetes.Interface, resource *appsv1.Deployment) error {
	return wait.Poll(1*time.Second, 5*time.Minute, func() (bool, error) {
		d, err := kubeClientSet.AppsV1().Deployments(resource.Namespace).Get(context.TODO(), resource.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return false, nil
		}

		if err != nil {
			return false, nil
		}

		if d.DeletionTimestamp != nil {
			return false, fmt.Errorf("deployment %q is being deleted", resource.Name)
		}

		if d.Generation <= d.Status.ObservedGeneration && d.Status.UpdatedReplicas == d.Status.Replicas && d.Status.UnavailableReplicas == 0 {
			return true, nil
		}

		return false, nil
	})
}
