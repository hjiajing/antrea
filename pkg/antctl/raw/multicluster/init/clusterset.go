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
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	multiclusterv1alpha1 "antrea.io/antrea/multicluster/apis/multicluster/v1alpha1"
)

func (d *deployer) deployClusterSet(cmd *cobra.Command, role string, clusterName string) error {
	fmt.Fprintf(cmd.OutOrStdout(), "Creating ClusterSet in %s cluster %s\n", role, clusterName)

	var client client.Client
	var namespace string
	if role == leaderRole {
		client = d.leaderClient
		namespace = d.leaderNamespace
	} else {
		client = d.memberClient[clusterName]
		namespace = d.memberNamespace
	}

	var createErr error
	clusterClaim := multiclusterv1alpha1.ClusterClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      multiclusterv1alpha1.WellKnownClusterClaimID,
			Namespace: namespace,
		},
		Name:  multiclusterv1alpha1.WellKnownClusterClaimID,
		Value: clusterName,
	}
	if createErr = client.Create(context.TODO(), &clusterClaim); createErr != nil {
		if errors.IsAlreadyExists(createErr) {
			fmt.Fprintf(cmd.OutOrStderr(), "ClusterClaim \"%s\" in Namespace %s of %s cluster already exists\n", multiclusterv1alpha1.WellKnownClusterClaimID, namespace, role)
			createErr = nil
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Failed to create ClusterClaim \"%s\", error: %s\n", clusterClaim.Name, createErr.Error())
			return createErr
		}
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "ClusterClaim \"%s\" created\n", multiclusterv1alpha1.WellKnownClusterClaimID)
		defer func(clusterClaim multiclusterv1alpha1.ClusterClaim) {
			if createErr != nil {
				if err := client.Delete(context.TODO(), &clusterClaim); err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "Failed to delete ClusterClaim \"%s\", error: %s\n", clusterClaim.Name, err.Error())
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "ClusterClaim \"%s\" deleted\n", clusterClaim.Name)
				}
			}
		}(clusterClaim)
	}

	clustersetClaim := multiclusterv1alpha1.ClusterClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      multiclusterv1alpha1.WellKnownClusterClaimClusterSet,
			Namespace: namespace,
		},
		Name:  multiclusterv1alpha1.WellKnownClusterClaimClusterSet,
		Value: clusterName,
	}
	if createErr = client.Create(context.TODO(), &clustersetClaim); createErr != nil {
		if errors.IsAlreadyExists(createErr) {
			fmt.Fprintf(cmd.OutOrStderr(), "ClusterClaim \"%s\" in Namespace %s in %s cluster already exists\n", multiclusterv1alpha1.WellKnownClusterClaimClusterSet, namespace, role)
			createErr = nil
		} else {
			return createErr
		}
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "ClusterClaim \"%s\" created\n", multiclusterv1alpha1.WellKnownClusterClaimClusterSet)
		defer func(clusterClaim multiclusterv1alpha1.ClusterClaim) {
			if createErr != nil {
				if err := client.Delete(context.TODO(), &clusterClaim); err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "Failed to delete ClusterClaim \"%s\", error: %s\n", clusterClaim.Name, err.Error())
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "ClusterClaim \"%s\" deleted\n", clusterClaim.Name)
				}
			}
		}(clustersetClaim)
	}

	clusterSet := multiclusterv1alpha1.ClusterSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: namespace,
		},
		Spec: multiclusterv1alpha1.ClusterSetSpec{
			Namespace: d.leaderNamespace,
		},
	}

	if role == memberRole {
		clusterSet.Spec.Leaders = append(clusterSet.Spec.Leaders, multiclusterv1alpha1.MemberCluster{
			ClusterID:      d.leaderCluster,
			Server:         d.leaderServer,
			Secret:         fmt.Sprintf("member-%s-access-token", clusterName),
			ServiceAccount: fmt.Sprintf("member-%s-access-sa", clusterName),
		})
		clusterSet.Spec.Members = append(clusterSet.Spec.Members, multiclusterv1alpha1.MemberCluster{
			ClusterID: clusterName,
		})
	} else {
		clusterSet.Spec.Leaders = append(clusterSet.Spec.Leaders, multiclusterv1alpha1.MemberCluster{
			ClusterID: d.leaderCluster,
		})
		for _, member := range d.memberClusters {
			clusterSet.Spec.Members = append(clusterSet.Spec.Members, multiclusterv1alpha1.MemberCluster{
				ClusterID:      member,
				ServiceAccount: fmt.Sprintf("member-%s-access-sa", member),
			})
		}
	}

	if createErr = client.Create(context.TODO(), &clusterSet); createErr != nil {
		if errors.IsAlreadyExists(createErr) {
			fmt.Fprintf(cmd.OutOrStderr(), "ClusterSet \"%s\" in Namespace %s in %s cluster already exists\n", clusterName, namespace, role)
			createErr = nil
		} else {
			return createErr
		}
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "ClusterSet \"%s\" created\n", clusterName)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "ClusterClaim and ClusterSet in cluster %s deployed\n", clusterName)

	return nil
}
