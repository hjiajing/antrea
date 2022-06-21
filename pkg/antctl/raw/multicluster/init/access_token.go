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
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (d *deployer) createAccessToken(cmd *cobra.Command) error {
	var createErr error
	for _, memberCluster := range d.memberClusters {
		fmt.Fprintf(cmd.OutOrStdout(), "Creating access token for member cluster %s\n", memberCluster)

		serviceAccountName := fmt.Sprintf("member-%s-access-sa", memberCluster)
		fmt.Fprintf(cmd.OutOrStdout(), "Creating ServiceAccount \"%s\" in leader cluster\n", serviceAccountName)
		serviceAccount := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceAccountName,
				Namespace: d.leaderNamespace,
			},
		}
		createErr = d.leaderClient.Create(context.TODO(), serviceAccount)
		if createErr != nil {
			if errors.IsAlreadyExists(createErr) {
				fmt.Fprintf(cmd.OutOrStderr(), "ServiceAccount \"%s\" already exists\n", serviceAccountName)
				createErr = nil
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Failed to create ServiceAccount \"%s\", error: %s\n", serviceAccountName, createErr.Error())
				return createErr
			}
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "ServiceAccount \"%s\" created\n", serviceAccount.Name)
			defer func(serviceAccount *corev1.ServiceAccount) {
				if createErr != nil {
					err := d.leaderClient.Delete(context.TODO(), serviceAccount)
					if err != nil {
						fmt.Fprintf(cmd.OutOrStdout(), "Failed to delete ServiceAccount \"%s\", error: %s\n", serviceAccount.Name, err.Error())
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "ServiceAccount \"%s\" deleted\n", serviceAccount.Name)
					}
				}
			}(serviceAccount)
		}

		roleBindingName := fmt.Sprintf("member-%s-rolebinding", memberCluster)
		fmt.Fprintf(cmd.OutOrStdout(), "Creating RoleBinding \"%s\" in leader cluster\n", roleBindingName)
		roleBinding := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      roleBindingName,
				Namespace: d.leaderNamespace,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "antrea-mc-member-cluster-role",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      serviceAccountName,
					Namespace: d.leaderNamespace,
				},
			},
		}

		createErr = d.leaderClient.Create(context.TODO(), roleBinding)
		if createErr != nil {
			if errors.IsAlreadyExists(createErr) {
				fmt.Fprintf(cmd.OutOrStderr(), "RoleBinding \"%s\" already exists\n", roleBindingName)
				createErr = nil
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Failed to create RoleBingding \"%s\", error: %s\n", roleBindingName, createErr.Error())
				return createErr
			}
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "RoleBinding \"%s\" created\n", roleBinding.Name)
			defer func(roleBinding *rbacv1.RoleBinding) {
				if createErr != nil {
					err := d.leaderClient.Delete(context.TODO(), roleBinding)
					if err != nil {
						fmt.Fprintf(cmd.OutOrStdout(), "Failed to delete RoleBinding \"%s\", error: %s\n", roleBinding.Name, err.Error())
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "RoleBinding \"%s\" deleted\n", roleBinding.Name)
					}
				}
			}(roleBinding)
		}

		secretName := fmt.Sprintf("member-%s-access-token", memberCluster)
		fmt.Fprintf(cmd.OutOrStdout(), "Creating Secret \"%s\" in leader cluster\n", secretName)
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: d.leaderNamespace,
				Annotations: map[string]string{
					"kubernetes.io/service-account.name": serviceAccount.Name,
				},
			},
			Type: "kubernetes.io/service-account-token",
		}

		createErr = d.leaderClient.Create(context.TODO(), secret)
		if createErr != nil {
			if errors.IsAlreadyExists(createErr) {
				fmt.Fprintf(cmd.OutOrStderr(), "Secret \"%s\" already exists\n", secretName)
				createErr = nil
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Failed to create Secret \"%s\", start rollback\n", secretName)
				return createErr
			}
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Secret \"%s\" created\n", secretName)
			defer func(secret *corev1.Secret) {
				if createErr != nil {
					err := d.leaderClient.Delete(context.TODO(), secret)
					if err != nil {
						fmt.Fprintf(cmd.OutOrStdout(), "Failed to delete Secret \"%s\", error: %s\n", secret.Name, err.Error())
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "Secret \"%s\" deleted\n", secret.Name)
					}
				}
			}(secret)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Creating access-token in member cluster %s\n", memberCluster)
		accessToken := &corev1.Secret{}
		if createErr = d.leaderClient.Get(context.TODO(), types.NamespacedName{Namespace: d.leaderNamespace, Name: secretName}, accessToken); createErr != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Failed to get Secret \"%s\" in leader cluster\n", secretName)
			return createErr
		}
		token := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      accessToken.Name,
				Namespace: d.memberNamespace,
			},
			Data: accessToken.Data,
			Type: "Opaque",
		}
		if createErr = d.memberClient[memberCluster].Create(context.TODO(), token); createErr != nil {
			if errors.IsAlreadyExists(createErr) {
				fmt.Fprintf(cmd.OutOrStderr(), "Secret \"%s\" already exists\n", secretName)
				createErr = nil
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Failed to create Secret \"%s\" in member cluster \"%s\"\n", token.Name, memberCluster)
			}
			return createErr
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Secret created in the leader and member clusters\n")

	return nil
}
