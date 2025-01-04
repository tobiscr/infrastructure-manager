package backup

import (
	"context"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	authenticationv1alpha1 "github.com/gardener/oidc-webhook-authenticator/apis/authentication/v1alpha1"
	"github.com/kyma-project/infrastructure-manager/hack/runtime-migrator-app/internal/initialisation"
	"github.com/kyma-project/infrastructure-manager/pkg/gardener/kubeconfig"
	"github.com/pkg/errors"
	rbacv1 "k8s.io/api/rbac/v1"
	crdv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Backuper struct {
	cfg                initialisation.Config
	isDryRun           bool
	kubeconfigProvider kubeconfig.Provider
	kcpClient          client.Client
}

func NewBackuper(isDryRun bool, kcpClient client.Client) Backuper {
	return Backuper{
		isDryRun:  isDryRun,
		kcpClient: kcpClient,
	}
}

type RuntimeBackup struct {
	OriginalShoot       v1beta1.Shoot
	ShootToRestore      v1beta1.Shoot
	ClusterRoleBindings []rbacv1.ClusterRoleBinding
	OIDCConfig          []authenticationv1alpha1.OpenIDConnect
}

func (b Backuper) Do(ctx context.Context, shoot v1beta1.Shoot, runtimeID string) (RuntimeBackup, error) {
	runtimeClient, err := initialisation.GetRuntimeClient(ctx, b.kcpClient, runtimeID)
	if err != nil {
		return RuntimeBackup{}, err
	}

	crbs, err := b.getCRBs(runtimeClient)
	if err != nil {
		return RuntimeBackup{}, errors.Wrap(err, "failed to get Cluster Role Bindings")
	}

	oidcConfig, err := b.getOIDCConfig(runtimeClient)
	if err != nil {
		return RuntimeBackup{}, errors.Wrap(err, "failed to get OIDC config")
	}

	return RuntimeBackup{
		ShootToRestore:      b.getShootToRestore(shoot),
		OriginalShoot:       shoot,
		ClusterRoleBindings: crbs,
		OIDCConfig:          oidcConfig,
	}, nil
}

func (b Backuper) getShootToRestore(shootFromGardener v1beta1.Shoot) v1beta1.Shoot {
	return v1beta1.Shoot{
		TypeMeta: v1.TypeMeta{
			Kind:       "Shoot",
			APIVersion: "core.gardener.cloud/v1beta1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:        shootFromGardener.Name,
			Namespace:   shootFromGardener.Namespace,
			Annotations: shootFromGardener.Annotations,
			Labels:      shootFromGardener.Labels,
		},
		Spec: v1beta1.ShootSpec{
			ControlPlane:      shootFromGardener.Spec.ControlPlane,
			CloudProfileName:  shootFromGardener.Spec.CloudProfileName,
			DNS:               shootFromGardener.Spec.DNS,
			Extensions:        shootFromGardener.Spec.Extensions,
			ExposureClassName: shootFromGardener.Spec.ExposureClassName,
			Kubernetes: v1beta1.Kubernetes{
				EnableStaticTokenKubeconfig: shootFromGardener.Spec.Kubernetes.EnableStaticTokenKubeconfig,
				KubeAPIServer: &v1beta1.KubeAPIServerConfig{
					// TODO: consider skipping ClientAuthentication
					OIDCConfig: shootFromGardener.Spec.Kubernetes.KubeAPIServer.OIDCConfig,
				},
				Version: shootFromGardener.Spec.Kubernetes.Version,
			},
			Maintenance: &v1beta1.Maintenance{
				AutoUpdate: shootFromGardener.Spec.Maintenance.AutoUpdate,
			},
			Networking: &v1beta1.Networking{
				Type:     shootFromGardener.Spec.Networking.Type,
				Nodes:    shootFromGardener.Spec.Networking.Nodes,
				Pods:     shootFromGardener.Spec.Networking.Pods,
				Services: shootFromGardener.Spec.Networking.Services,
			},
			// TODO: consider if we need to do the backup selectively (workers)
			Provider:          shootFromGardener.Spec.Provider,
			Purpose:           shootFromGardener.Spec.Purpose,
			Region:            shootFromGardener.Spec.Region,
			Resources:         shootFromGardener.Spec.Resources,
			SecretBindingName: shootFromGardener.Spec.SecretBindingName,
			SeedSelector:      shootFromGardener.Spec.SeedSelector,
		},
	}
}

func (b Backuper) getCRBs(runtimeClient client.Client) ([]rbacv1.ClusterRoleBinding, error) {
	var crbList rbacv1.ClusterRoleBindingList
	err := runtimeClient.List(context.Background(), &crbList)

	if err != nil {
		return nil, err
	}

	crbsToBackup := make([]rbacv1.ClusterRoleBinding, 0)

	for _, crb := range crbList.Items {
		if crb.RoleRef.Kind == "ClusterRole" && crb.RoleRef.Name == "cluster-admin" {
			crbsToBackup = append(crbsToBackup, crb)
		}
	}

	return crbsToBackup, nil
}

func (b Backuper) getOIDCConfig(runtimeClient client.Client) ([]authenticationv1alpha1.OpenIDConnect, error) {
	found, err := oidcCRDExists(runtimeClient)
	if err != nil {
		return nil, err
	}

	if found {
		var oidcConfigList authenticationv1alpha1.OpenIDConnectList

		err := runtimeClient.List(context.Background(), &oidcConfigList)

		if err != nil {
			return nil, err
		}

		return oidcConfigList.Items, nil
	}

	return []authenticationv1alpha1.OpenIDConnect{}, nil
}

func oidcCRDExists(runtimeClient client.Client) (bool, error) {
	var crdsList crdv1.CustomResourceDefinitionList
	err := runtimeClient.List(context.Background(), &crdsList)
	if err != nil {
		return false, err
	}

	for _, crd := range crdsList.Items {
		if crd.Name == "openidconnects.authentication.gardener.cloud" {
			return true, nil
		}
	}

	return false, nil
}
