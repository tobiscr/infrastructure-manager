package extender

import (
	"encoding/json"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	aws "github.com/kyma-project/infrastructure-manager/pkg/gardener/shoot/hyperscaler/aws"
	"github.com/kyma-project/infrastructure-manager/pkg/gardener/shoot/hyperscaler/azure"
	"k8s.io/apimachinery/pkg/runtime"

	"testing"

	"github.com/gardener/gardener-extension-provider-aws/pkg/apis/aws/v1alpha1"
	gardener "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	imv1 "github.com/kyma-project/infrastructure-manager/api/v1"
	"github.com/kyma-project/infrastructure-manager/pkg/gardener/shoot/hyperscaler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderExtenderForCreateAWS(t *testing.T) {
	// tests of ProviderExtenderForCreateOperation for AWS provider and single worker
	for tname, tc := range map[string]struct {
		Runtime                     imv1.Runtime
		EnableIMDSv2                bool
		DefaultMachineImageVersion  string
		ExpectedMachineImageVersion string
		DefaultMachineImageName     string
		ExpectedMachineImageName    string
		CurrentZonesConfig          []string
		ExpectedZonesCount          int
	}{
		"Create provider specific config for AWS without worker config and one zone": {
			Runtime: imv1.Runtime{
				Spec: imv1.RuntimeSpec{
					Shoot: imv1.RuntimeShoot{
						Provider: fixProvider(hyperscaler.TypeAWS, "gardenlinux", "1312.2.0", []string{"eu-central-1a"}),
					},
				},
			},
			EnableIMDSv2:                false,
			DefaultMachineImageVersion:  "1312.3.0",
			ExpectedMachineImageVersion: "1312.2.0",
			ExpectedMachineImageName:    "gardenlinux",
			ExpectedZonesCount:          1,
		},
		"Create provider specific config for AWS without worker config and two zones": {
			Runtime: imv1.Runtime{
				Spec: imv1.RuntimeSpec{
					Shoot: imv1.RuntimeShoot{
						Provider: fixProvider(hyperscaler.TypeAWS, "gardenlinux", "1312.2.0", []string{"eu-central-1a", "eu-central-1b"}),
					},
				},
			},
			EnableIMDSv2:                false,
			DefaultMachineImageVersion:  "1312.3.0",
			ExpectedMachineImageVersion: "1312.2.0",
			ExpectedMachineImageName:    "gardenlinux",
			ExpectedZonesCount:          2,
		},
		"Create provider specific config for AWS without worker config and three zones": {
			Runtime: imv1.Runtime{
				Spec: imv1.RuntimeSpec{
					Shoot: imv1.RuntimeShoot{
						Provider: fixProvider(hyperscaler.TypeAWS, "gardenlinux", "1312.2.0", []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}),
					},
				},
			},
			EnableIMDSv2:                false,
			DefaultMachineImageVersion:  "1312.3.0",
			ExpectedMachineImageVersion: "1312.2.0",
			ExpectedMachineImageName:    "gardenlinux",
			ExpectedZonesCount:          3,
		},
		"Create provider config for AWS with worker config and three zones": {
			Runtime: imv1.Runtime{
				Spec: imv1.RuntimeSpec{
					Shoot: imv1.RuntimeShoot{
						Provider: fixProvider(hyperscaler.TypeAWS, "", "", []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}),
					},
				},
			},
			EnableIMDSv2:                true,
			DefaultMachineImageVersion:  "1312.3.0",
			ExpectedMachineImageVersion: "1312.3.0",
			ExpectedZonesCount:          3,
		},
	} {
		t.Run(tname, func(t *testing.T) {
			// given
			shoot := fixEmptyGardenerShoot("cluster", "kcp-system")

			// when

			extender := NewProviderExtenderForCreateOperation(tc.EnableIMDSv2, tc.DefaultMachineImageName, tc.DefaultMachineImageVersion)
			err := extender(tc.Runtime, &shoot)

			// then
			require.NoError(t, err)

			assertProvider(t, tc.Runtime.Spec.Shoot, shoot, tc.EnableIMDSv2, tc.ExpectedMachineImageName, tc.ExpectedMachineImageVersion)
			assertProviderSpecificConfigAWS(t, shoot, tc.ExpectedZonesCount)
		})
	}

	t.Run("Return error for unknown provider", func(t *testing.T) {
		// given
		shoot := fixEmptyGardenerShoot("cluster", "kcp-system")
		runtime := imv1.Runtime{
			Spec: imv1.RuntimeSpec{
				Shoot: imv1.RuntimeShoot{
					Provider: imv1.Provider{
						Type: "unknown",
					},
				},
			},
		}

		// when
		extender := NewProviderExtenderForCreateOperation(false, "", "")
		err := extender(runtime, &shoot)

		// then
		require.Error(t, err)
	})
}

func TestProviderExtenderForPatchSingleWorkerAWS(t *testing.T) {
	// tests of NewProviderExtenderPatch for provider image version patching AWS only operation is provider-agnostic
	for tname, tc := range map[string]struct {
		Runtime                     imv1.Runtime
		EnableIMDSv2                bool
		DefaultMachineImageVersion  string
		ExpectedMachineImageVersion string
		DefaultMachineImageName     string
		ExpectedMachineImageName    string
		CurrentShootWorkers         []gardener.Worker
		ExistingInfraConfig         *runtime.RawExtension
		ExistingControlPlaneConfig  *runtime.RawExtension
		ExpectedShootWorkersCount   int
		ExpectedZonesCount          int
	}{
		"Same image name - use bigger current shoot machine image version as image version": {
			Runtime: imv1.Runtime{
				Spec: imv1.RuntimeSpec{
					Shoot: imv1.RuntimeShoot{
						Provider: fixProvider(hyperscaler.TypeAWS, "gardenlinux", "1312.2.0", []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}),
						Networking: imv1.Networking{
							Nodes: "10.250.0.0/22",
						},
					},
				},
			},
			EnableIMDSv2:                false,
			DefaultMachineImageName:     "gardenlinux",
			DefaultMachineImageVersion:  "1312.3.0",
			ExpectedMachineImageVersion: "1312.4.0",
			ExpectedZonesCount:          3,
			ExpectedMachineImageName:    "gardenlinux",
			CurrentShootWorkers:         fixWorkers("worker", "m6i.large", "gardenlinux", "1312.4.0", 1, 3, []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}),
			ExistingInfraConfig:         fixAWSInfrastructureConfig("10.250.0.0/22", []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}),
			ExistingControlPlaneConfig:  fixAWSControlPlaneConfig(),
		},
		"Same image name - override current shoot machine image version with new bigger version from RuntimeCR": {
			Runtime: imv1.Runtime{
				Spec: imv1.RuntimeSpec{
					Shoot: imv1.RuntimeShoot{
						Provider: fixProvider(hyperscaler.TypeAWS, "gardenlinux", "1312.2.0", []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}),
					},
				},
			},
			EnableIMDSv2:                false,
			DefaultMachineImageName:     "gardenlinux",
			DefaultMachineImageVersion:  "1312.3.0",
			CurrentShootWorkers:         fixWorkers("worker", "m6i.large", "gardenlinux", "1312.1.0", 1, 3, []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}),
			ExpectedMachineImageVersion: "1312.2.0",
			ExpectedZonesCount:          3,
			ExpectedMachineImageName:    "gardenlinux",
			ExistingInfraConfig:         fixAWSInfrastructureConfig("10.250.0.0/22", []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}),
			ExistingControlPlaneConfig:  fixAWSControlPlaneConfig(),
		},
		"Same image name - no version is provided override current shoot machine image version with default version": {
			Runtime: imv1.Runtime{
				Spec: imv1.RuntimeSpec{
					Shoot: imv1.RuntimeShoot{
						Provider: fixProvider(hyperscaler.TypeAWS, "", "", []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}),
					},
				},
			},
			EnableIMDSv2:                false,
			DefaultMachineImageName:     "gardenlinux",
			DefaultMachineImageVersion:  "1312.3.0",
			CurrentShootWorkers:         fixWorkers("worker", "m6i.large", "gardenlinux", "1312.1.0", 1, 3, []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}),
			ExpectedMachineImageVersion: "1312.3.0",
			ExpectedZonesCount:          3,
			ExpectedMachineImageName:    "gardenlinux",
			ExistingInfraConfig:         fixAWSInfrastructureConfig("10.250.0.0/22", []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}),
			ExistingControlPlaneConfig:  fixAWSControlPlaneConfig(),
		},
		"Different image name - override current shoot machine image and version with new data from RuntimeCR": {
			Runtime: imv1.Runtime{
				Spec: imv1.RuntimeSpec{
					Shoot: imv1.RuntimeShoot{
						Provider: fixProvider(hyperscaler.TypeAWS, "gardenlinux", "1312.2.0", []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}),
					},
				},
			},
			EnableIMDSv2:                false,
			DefaultMachineImageName:     "gardenlinux",
			DefaultMachineImageVersion:  "1312.3.0",
			CurrentShootWorkers:         fixWorkers("worker", "m6i.large", "ubuntu", "1312.4.0", 1, 3, []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}),
			ExpectedZonesCount:          3,
			ExpectedMachineImageName:    "gardenlinux",
			ExpectedMachineImageVersion: "1312.2.0",
			ExistingInfraConfig:         fixAWSInfrastructureConfig("10.250.0.0/22", []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}),
			ExistingControlPlaneConfig:  fixAWSControlPlaneConfig(),
		},
		"Different image name - no data is provided override current shoot machine image and version with default data": {
			Runtime: imv1.Runtime{
				Spec: imv1.RuntimeSpec{
					Shoot: imv1.RuntimeShoot{
						Provider: fixProvider(hyperscaler.TypeAWS, "", "", []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}),
					},
				},
			},
			EnableIMDSv2:                false,
			DefaultMachineImageName:     "gardenlinux",
			DefaultMachineImageVersion:  "1312.3.0",
			CurrentShootWorkers:         fixWorkers("worker", "m6i.large", "ubuntu", "1312.4.0", 1, 3, []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}),
			ExpectedMachineImageVersion: "1312.3.0",
			ExpectedZonesCount:          3,
			ExpectedMachineImageName:    "gardenlinux",
			ExistingInfraConfig:         fixAWSInfrastructureConfig("10.250.0.0/22", []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}),
			ExistingControlPlaneConfig:  fixAWSControlPlaneConfig(),
		},
		"Wrong current image name - use data from RuntimeCR": {
			Runtime: imv1.Runtime{
				Spec: imv1.RuntimeSpec{
					Shoot: imv1.RuntimeShoot{
						Provider: fixProvider(hyperscaler.TypeAWS, "gardenlinux", "1312.2.0", []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}),
					},
				},
			},
			EnableIMDSv2:                false,
			DefaultMachineImageName:     "gardenlinux",
			DefaultMachineImageVersion:  "1312.3.0",
			CurrentShootWorkers:         fixWorkers("worker", "m6i.large", "", "1312.4.0", 1, 3, []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}),
			ExpectedMachineImageVersion: "1312.2.0",
			ExpectedZonesCount:          3,
			ExpectedMachineImageName:    "gardenlinux",
			ExistingInfraConfig:         fixAWSInfrastructureConfig("10.250.0.0/22", []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}),
			ExistingControlPlaneConfig:  fixAWSControlPlaneConfig(),
		},
		"Wrong current image version - use data from RuntimeCR": {
			Runtime: imv1.Runtime{
				Spec: imv1.RuntimeSpec{
					Shoot: imv1.RuntimeShoot{
						Provider: fixProvider(hyperscaler.TypeAWS, "gardenlinux", "1312.2.0", []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}),
					},
				},
			},
			EnableIMDSv2:                false,
			DefaultMachineImageName:     "gardenlinux",
			DefaultMachineImageVersion:  "1312.3.0",
			CurrentShootWorkers:         fixWorkers("worker", "m6i.large", "gardenlinux", "", 1, 3, []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}),
			ExpectedMachineImageVersion: "1312.2.0",
			ExpectedZonesCount:          3,
			ExpectedMachineImageName:    "gardenlinux",
			ExistingInfraConfig:         fixAWSInfrastructureConfig("10.250.0.0/22", []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}),
			ExistingControlPlaneConfig:  fixAWSControlPlaneConfig(),
		},
		// TODO: Add tests for override of infrastructure and control plane config with values from RuntimeCR
	} {
		t.Run(tname, func(t *testing.T) {
			// given
			shoot := fixEmptyGardenerShoot("cluster", "kcp-system")

			// when
			extender := NewProviderExtenderPatchOperation(tc.EnableIMDSv2, tc.DefaultMachineImageName, tc.DefaultMachineImageVersion, tc.CurrentShootWorkers, tc.ExistingInfraConfig, tc.ExistingControlPlaneConfig)
			err := extender(tc.Runtime, &shoot)

			// then
			require.NoError(t, err)

			assertProvider(t, tc.Runtime.Spec.Shoot, shoot, tc.EnableIMDSv2, tc.ExpectedMachineImageName, tc.ExpectedMachineImageVersion)
			assertProviderSpecificConfigAWS(t, shoot, tc.ExpectedZonesCount)
		})
	}
}

type workerConfig struct {
	Name                string
	MachineType         string
	MachineImageName    string
	MachineImageVersion string
	hypescalerMin       int32
	hyperscalerMax      int32
	Zones               []string
}

func TestProviderExtenderForCreateMultipleWorkersAWS(t *testing.T) {
	// tests of NewProviderExtenderForCreateOperation for workers create operation
	for tname, tc := range map[string]struct {
		Runtime                    imv1.Runtime
		EnableIMDSv2               bool
		DefaultMachineImageVersion string
		DefaultMachineImageName    string
		CurrentShootWorkers        []gardener.Worker
		ExistingInfraConfig        *runtime.RawExtension
		ExistingControlPlaneConfig *runtime.RawExtension
		ExpectedShootWorkers       []gardener.Worker
		ExpectedZonesCount         int
	}{
		"Create provider config for multiple workers without AWS worker config": {
			Runtime: imv1.Runtime{
				Spec: imv1.RuntimeSpec{
					Shoot: imv1.RuntimeShoot{
						Provider: fixProviderWithMultipleWorkers(hyperscaler.TypeAWS, fixMultipleWorkers([]workerConfig{
							{"main-worker", "m6i.large", "gardenlinux", "1310.4.0", 1, 3, []string{"eu-central-1a"}},
							{"additional", "m7i.large", "gardenlinux", "1311.2.0", 2, 4, []string{"eu-central-1b", "eu-central-1c"}},
							{"another", "m8i.large", "gardenlinux", "1312.2.0", 3, 5, []string{"eu-central-1c"}},
						})),
						Networking: imv1.Networking{
							Nodes: "10.250.0.0/22",
						},
					},
				},
			},
			EnableIMDSv2:               false,
			DefaultMachineImageName:    "gardenlinux",
			DefaultMachineImageVersion: "1312.3.0",
			ExpectedZonesCount:         3,
			ExpectedShootWorkers: fixMultipleWorkers([]workerConfig{
				{"main-worker", "m6i.large", "gardenlinux", "1310.4.0", 1, 3, []string{"eu-central-1a"}},
				{"additional", "m7i.large", "gardenlinux", "1311.2.0", 2, 4, []string{"eu-central-1b", "eu-central-1c"}},
				{"another", "m8i.large", "gardenlinux", "1312.2.0", 3, 5, []string{"eu-central-1c"}}}),
		},
		"Create provider config for multiple workers with AWS worker config": {
			Runtime: imv1.Runtime{
				Spec: imv1.RuntimeSpec{
					Shoot: imv1.RuntimeShoot{
						Provider: fixProviderWithMultipleWorkers(hyperscaler.TypeAWS, fixMultipleWorkers([]workerConfig{
							{"main-worker", "m6i.large", "gardenlinux", "1310.4.0", 1, 3, []string{"eu-central-1a"}},
							{"additional", "m7i.large", "gardenlinux", "1311.2.0", 2, 4, []string{"eu-central-1b", "eu-central-1c"}},
							{"another", "m8i.large", "gardenlinux", "1312.2.0", 3, 5, []string{"eu-central-1c"}},
						})),
						Networking: imv1.Networking{
							Nodes: "10.250.0.0/22",
						},
					},
				},
			},
			EnableIMDSv2:               true,
			DefaultMachineImageName:    "gardenlinux",
			DefaultMachineImageVersion: "1312.3.0",
			ExpectedZonesCount:         3,
			ExpectedShootWorkers: fixMultipleWorkers([]workerConfig{
				{"main-worker", "m6i.large", "gardenlinux", "1310.4.0", 1, 3, []string{"eu-central-1a"}},
				{"additional", "m7i.large", "gardenlinux", "1311.2.0", 2, 4, []string{"eu-central-1b", "eu-central-1c"}},
				{"another", "m8i.large", "gardenlinux", "1312.2.0", 3, 5, []string{"eu-central-1c"}}}),
		},
	} {
		t.Run(tname, func(t *testing.T) {
			// given
			shoot := fixEmptyGardenerShoot("cluster", "kcp-system")

			// when
			extender := NewProviderExtenderForCreateOperation(tc.EnableIMDSv2, tc.DefaultMachineImageName, tc.DefaultMachineImageVersion)
			err := extender(tc.Runtime, &shoot)

			// then
			require.NoError(t, err)

			assertProviderMultipleWorkers(t, tc.Runtime.Spec.Shoot, shoot, tc.EnableIMDSv2, tc.ExpectedShootWorkers)
			assertProviderSpecificConfigAWS(t, shoot, tc.ExpectedZonesCount)
		})
	}
}

func TestProviderExtenderForPatchWorkersUpdateAWS(t *testing.T) {
	// tests of NewProviderExtenderPatch for workers update operation
	for tname, tc := range map[string]struct {
		Runtime                    imv1.Runtime
		EnableIMDSv2               bool
		DefaultMachineImageVersion string
		DefaultMachineImageName    string
		CurrentShootWorkers        []gardener.Worker
		ExistingInfraConfig        *runtime.RawExtension
		ExistingControlPlaneConfig *runtime.RawExtension
		ExpectedShootWorkers       []gardener.Worker
		ExpectedZonesCount         int
	}{
		"Add additional worker": {
			Runtime: imv1.Runtime{
				Spec: imv1.RuntimeSpec{
					Shoot: imv1.RuntimeShoot{
						Provider: fixProviderWithMultipleWorkers(hyperscaler.TypeAWS, fixMultipleWorkers([]workerConfig{
							{"main-worker", "m6i.large", "gardenlinux", "1312.4.0", 1, 3, []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}},
							{"additional", "m6i.large", "gardenlinux", "1312.2.0", 1, 3, []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}},
						})),
						Networking: imv1.Networking{
							Nodes: "10.250.0.0/22",
						},
					},
				},
			},
			EnableIMDSv2:               false,
			DefaultMachineImageName:    "gardenlinux",
			DefaultMachineImageVersion: "1312.3.0",
			ExpectedZonesCount:         3,
			CurrentShootWorkers:        fixWorkers("main-worker", "m6i.large", "gardenlinux", "1312.4.0", 1, 3, []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}),
			ExpectedShootWorkers: fixMultipleWorkers([]workerConfig{
				{"main-worker", "m6i.large", "gardenlinux", "1312.4.0", 1, 3, []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}},
				{"additional", "m6i.large", "gardenlinux", "1312.2.0", 1, 3, []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}}}),
			ExistingInfraConfig:        fixAWSInfrastructureConfig("10.250.0.0/22", []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}),
			ExistingControlPlaneConfig: fixAWSControlPlaneConfig(),
		},
		"Remove additional worker from existing set of workers": {
			Runtime: imv1.Runtime{
				Spec: imv1.RuntimeSpec{
					Shoot: imv1.RuntimeShoot{
						Provider: fixProviderWithMultipleWorkers(hyperscaler.TypeAWS, fixMultipleWorkers([]workerConfig{
							{"main-worker", "m6i.large", "gardenlinux", "1312.4.0", 1, 3, []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}}})),
						Networking: imv1.Networking{
							Nodes: "10.250.0.0/22",
						},
					},
				},
			},
			EnableIMDSv2:               false,
			DefaultMachineImageName:    "gardenlinux",
			DefaultMachineImageVersion: "1312.3.0",
			ExpectedZonesCount:         3,
			CurrentShootWorkers: fixMultipleWorkers([]workerConfig{
				{"main-worker", "m6i.large", "gardenlinux", "1312.4.0", 1, 3, []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}},
				{"additional", "m6i.large", "gardenlinux", "1312.2.0", 1, 3, []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}}}),
			ExpectedShootWorkers: fixMultipleWorkers([]workerConfig{
				{"main-worker", "m6i.large", "gardenlinux", "1312.4.0", 1, 3, []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}}}),
			ExistingInfraConfig:        fixAWSInfrastructureConfig("10.250.0.0/22", []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}),
			ExistingControlPlaneConfig: fixAWSControlPlaneConfig(),
		},
		"Update machine image name and version in multiple workers separately": {
			Runtime: imv1.Runtime{
				Spec: imv1.RuntimeSpec{
					Shoot: imv1.RuntimeShoot{
						Provider: fixProviderWithMultipleWorkers(hyperscaler.TypeAWS, fixMultipleWorkers([]workerConfig{
							{"main-worker", "m6i.large", "gardenlinux", "1313.4.0", 1, 3, []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}},
							{"additional", "m6i.large", "gardenlinux", "1313.2.0", 1, 3, []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}},
						})),
						Networking: imv1.Networking{
							Nodes: "10.250.0.0/22",
						},
					},
				},
			},
			EnableIMDSv2:               false,
			DefaultMachineImageName:    "gardenlinux",
			DefaultMachineImageVersion: "1312.3.0",
			CurrentShootWorkers: fixMultipleWorkers([]workerConfig{
				{"main-worker", "m6i.large", "gardenlinux", "1312.4.0", 1, 3, []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}},
				{"additional", "m6i.large", "gardenlinux", "1312.2.0", 1, 3, []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}}}),
			ExpectedShootWorkers: fixMultipleWorkers([]workerConfig{
				{"main-worker", "m6i.large", "gardenlinux", "1313.4.0", 1, 3, []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}},
				{"additional", "m6i.large", "gardenlinux", "1313.2.0", 1, 3, []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}}}),
			ExpectedZonesCount:         3,
			ExistingInfraConfig:        fixAWSInfrastructureConfig("10.250.0.0/22", []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}),
			ExistingControlPlaneConfig: fixAWSControlPlaneConfig(),
		},
		"Remove worker from existing set of workers networking zones set in infrastructureConfig should not change": {
			Runtime: imv1.Runtime{
				Spec: imv1.RuntimeSpec{
					Shoot: imv1.RuntimeShoot{
						Provider: fixProviderWithMultipleWorkers(hyperscaler.TypeAWS, fixMultipleWorkers([]workerConfig{
							{"main-worker", "m6i.large", "gardenlinux", "1313.4.0", 1, 3, []string{"eu-central-1a"}},
						})),
						Networking: imv1.Networking{
							Nodes: "10.250.0.0/22",
						},
					},
				},
			},
			EnableIMDSv2:               false,
			DefaultMachineImageName:    "gardenlinux",
			DefaultMachineImageVersion: "1312.3.0",
			CurrentShootWorkers: fixMultipleWorkers([]workerConfig{
				{"main-worker", "m6i.large", "gardenlinux", "1312.4.0", 1, 3, []string{"eu-central-1a"}},
				{"additional", "m6i.large", "gardenlinux", "1312.2.0", 1, 3, []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}}}),
			ExpectedShootWorkers: fixMultipleWorkers([]workerConfig{
				{"main-worker", "m6i.large", "gardenlinux", "1313.4.0", 1, 3, []string{"eu-central-1a"}}}),
			ExpectedZonesCount:         3,
			ExistingInfraConfig:        fixAWSInfrastructureConfig("10.250.0.0/22", []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}),
			ExistingControlPlaneConfig: fixAWSControlPlaneConfig(),
		},
	} {
		t.Run(tname, func(t *testing.T) {
			// given
			shoot := fixEmptyGardenerShoot("cluster", "kcp-system")

			// when
			extender := NewProviderExtenderPatchOperation(tc.EnableIMDSv2, tc.DefaultMachineImageName, tc.DefaultMachineImageVersion, tc.CurrentShootWorkers, tc.ExistingInfraConfig, tc.ExistingControlPlaneConfig)
			err := extender(tc.Runtime, &shoot)

			// then
			require.NoError(t, err)

			assertProviderMultipleWorkers(t, tc.Runtime.Spec.Shoot, shoot, tc.EnableIMDSv2, tc.ExpectedShootWorkers)
			assertProviderSpecificConfigAWS(t, shoot, tc.ExpectedZonesCount)
		})
	}
}

func TestProviderExtenderForCreateAzure(t *testing.T) {
	// tests of ProviderExtenderForCreateOperation for Azure provider
	for tname, tc := range map[string]struct {
		Runtime                     imv1.Runtime
		DefaultMachineImageVersion  string
		ExpectedMachineImageVersion string
		DefaultMachineImageName     string
		ExpectedMachineImageName    string
		CurrentZonesConfig          []string
		ExpectedZonesCount          int
	}{
		"Create provider specific config for Azure without worker config and one zone": {
			Runtime: imv1.Runtime{
				Spec: imv1.RuntimeSpec{
					Shoot: imv1.RuntimeShoot{
						Provider: fixProvider(hyperscaler.TypeAzure, "ubuntu", "18.04", []string{"1"}),
						Networking: imv1.Networking{
							Nodes: "10.250.0.0/22",
						},
					},
				},
			},
			DefaultMachineImageVersion:  "18.04-LTS",
			ExpectedMachineImageVersion: "18.04",
			ExpectedMachineImageName:    "ubuntu",
			ExpectedZonesCount:          1,
		},
		"Create provider specific config for Azure without worker config and two zones": {
			Runtime: imv1.Runtime{
				Spec: imv1.RuntimeSpec{
					Shoot: imv1.RuntimeShoot{
						Provider: fixProvider(hyperscaler.TypeAzure, "ubuntu", "18.04", []string{"1", "2"}),
						Networking: imv1.Networking{
							Nodes: "10.250.0.0/22",
						},
					},
				},
			},
			DefaultMachineImageVersion:  "18.04-LTS",
			ExpectedMachineImageVersion: "18.04",
			ExpectedMachineImageName:    "ubuntu",
			ExpectedZonesCount:          2,
		},
		"Create provider specific config for Azure without worker config and three zones": {
			Runtime: imv1.Runtime{
				Spec: imv1.RuntimeSpec{
					Shoot: imv1.RuntimeShoot{
						Provider: fixProvider(hyperscaler.TypeAzure, "ubuntu", "18.04", []string{"1", "2", "3"}),
						Networking: imv1.Networking{
							Nodes: "10.250.0.0/22",
						},
					},
				},
			},
			DefaultMachineImageVersion:  "18.04-LTS",
			ExpectedMachineImageVersion: "18.04",
			ExpectedMachineImageName:    "ubuntu",
			ExpectedZonesCount:          3,
		},
		"Create provider specific config for Azure with worker config and three zones": {
			Runtime: imv1.Runtime{
				Spec: imv1.RuntimeSpec{
					Shoot: imv1.RuntimeShoot{
						Provider: fixProvider(hyperscaler.TypeAzure, "", "", []string{"1", "2", "3"}),
						Networking: imv1.Networking{
							Nodes: "10.250.0.0/22",
						},
					},
				},
			},
			DefaultMachineImageVersion:  "18.04-LTS",
			ExpectedMachineImageVersion: "18.04-LTS",
			ExpectedZonesCount:          3,
		},
	} {
		t.Run(tname, func(t *testing.T) {
			// given
			shoot := fixEmptyGardenerShoot("cluster", "kcp-system")

			// when

			extender := NewProviderExtenderForCreateOperation(false, tc.DefaultMachineImageName, tc.DefaultMachineImageVersion)
			err := extender(tc.Runtime, &shoot)

			// then
			require.NoError(t, err)

			assertProvider(t, tc.Runtime.Spec.Shoot, shoot, false, tc.ExpectedMachineImageName, tc.ExpectedMachineImageVersion)
			assertProviderSpecificConfigAzure(t, shoot, tc.ExpectedZonesCount)
		})
	}
}

func TestProviderExtenderForCreateMultipleWorkersAzure(t *testing.T) {
	// tests of NewProviderExtenderForCreateOperation for workers create operation
	for tname, tc := range map[string]struct {
		Runtime                    imv1.Runtime
		DefaultMachineImageVersion string
		DefaultMachineImageName    string
		CurrentShootWorkers        []gardener.Worker
		ExistingInfraConfig        *runtime.RawExtension
		ExistingControlPlaneConfig *runtime.RawExtension
		ExpectedShootWorkers       []gardener.Worker
		ExpectedZonesCount         int
	}{
		"Create multiple workers without worker config": {
			Runtime: imv1.Runtime{
				Spec: imv1.RuntimeSpec{
					Shoot: imv1.RuntimeShoot{
						Provider: fixProviderWithMultipleWorkers(hyperscaler.TypeAzure, fixMultipleWorkers([]workerConfig{
							{"main-worker", "m6i.large", "gardenlinux", "1310.4.0", 1, 3, []string{"1"}},
							{"additional", "m7i.large", "gardenlinux", "1311.2.0", 2, 4, []string{"2", "3"}},
							{"another", "m8i.large", "gardenlinux", "1312.2.0", 3, 5, []string{"3"}},
						})),
						Networking: imv1.Networking{
							Nodes: "10.250.0.0/22",
						},
					},
				},
			},
			DefaultMachineImageName:    "gardenlinux",
			DefaultMachineImageVersion: "1312.3.0",
			ExpectedZonesCount:         3,
			ExpectedShootWorkers: fixMultipleWorkers([]workerConfig{
				{"main-worker", "m6i.large", "gardenlinux", "1310.4.0", 1, 3, []string{"1"}},
				{"additional", "m7i.large", "gardenlinux", "1311.2.0", 2, 4, []string{"2", "3"}},
				{"another", "m8i.large", "gardenlinux", "1312.2.0", 3, 5, []string{"3"}}}),
		},
	} {
		t.Run(tname, func(t *testing.T) {
			// given
			shoot := fixEmptyGardenerShoot("cluster", "kcp-system")

			// when
			extender := NewProviderExtenderForCreateOperation(false, tc.DefaultMachineImageName, tc.DefaultMachineImageVersion)
			err := extender(tc.Runtime, &shoot)

			// then
			require.NoError(t, err)

			assertProviderMultipleWorkers(t, tc.Runtime.Spec.Shoot, shoot, false, tc.ExpectedShootWorkers)
			assertProviderSpecificConfigAzure(t, shoot, tc.ExpectedZonesCount)
		})
	}
}

/*func TestProviderExtenderForCreateGCP(t *testing.T) {
	TBD
}

func TestProviderExtenderForCreateOpenStack(t *testing.T) {
	TBD
}

TestProviderExtenderForPatchAWSWorkers(t *testing.T) {
	TBD
}
*/

func fixProvider(providerType string, machineImageName, machineImageVersion string, zones []string) imv1.Provider {
	return imv1.Provider{
		Type: providerType,
		Workers: []gardener.Worker{
			{
				Name: "worker",
				Machine: gardener.Machine{
					Type:  "m6i.large",
					Image: fixMachineImage(machineImageName, machineImageVersion),
				},
				Minimum: 1,
				Maximum: 3,
				Zones:   zones,
			},
		},
	}
}

func fixWorkers(name, machineType, machineImageName, machineImageVersion string, min, max int32, zones []string) []gardener.Worker {
	return []gardener.Worker{
		{
			Name: name,
			Machine: gardener.Machine{
				Type: machineType,
				Image: &gardener.ShootMachineImage{
					Name:    machineImageName,
					Version: &machineImageVersion,
				},
			},
			Minimum: min,
			Maximum: max,
			Zones:   zones,
		},
	}
}

func fixMultipleWorkers(workers []workerConfig) []gardener.Worker {
	var result []gardener.Worker
	for _, w := range workers {
		result = append(result, gardener.Worker{
			Name: w.Name,
			Machine: gardener.Machine{
				Type:  w.MachineType,
				Image: fixMachineImage(w.MachineImageName, w.MachineImageVersion),
			},
			Minimum: w.hypescalerMin,
			Maximum: w.hyperscalerMax,
			Zones:   w.Zones,
		})
	}
	return result
}

func fixProviderWithMultipleWorkers(providerType string, workers []gardener.Worker) imv1.Provider {
	if len(workers) < 1 {
		return imv1.Provider{
			Type: providerType,
		}
	}

	var addPtr *[]gardener.Worker

	if len(workers) > 1 {
		additional := workers[1:]
		addPtr = &additional
	}

	return imv1.Provider{
		Type:              providerType,
		Workers:           workers[:1],
		AdditionalWorkers: addPtr,
	}
}

func fixProviderWithInfrastructureConfig(providerType string, machineImageName, machineImageVersion string, zones []string) imv1.Provider {
	var infraConfigBytes []byte
	switch providerType {
	case hyperscaler.TypeAWS:
		infraConfigBytes, _ = aws.GetInfrastructureConfig("10.250.0.0/22", zones)
	case hyperscaler.TypeAzure:
		infraConfigBytes, _ = azure.GetInfrastructureConfig("10.250.0.0/22", zones)
	default:
		panic("unknown provider type")
	}

	return imv1.Provider{
		Type: providerType,
		Workers: []gardener.Worker{
			{
				Name: "worker",
				Machine: gardener.Machine{
					Type:  "m6i.large",
					Image: fixMachineImage(machineImageName, machineImageVersion),
				},
				Minimum: 1,
				Maximum: 3,
				Zones:   zones,
			},
		},
		InfrastructureConfig: &runtime.RawExtension{Raw: infraConfigBytes},
	}
}

func fixAWSInfrastructureConfig(workersCIDR string, zones []string) *runtime.RawExtension {
	infraConfig, _ := aws.GetInfrastructureConfig(workersCIDR, zones)
	return &runtime.RawExtension{Raw: infraConfig}
}

func fixAWSControlPlaneConfig() *runtime.RawExtension {
	controlPlaneConfig, _ := aws.GetControlPlaneConfig([]string{})
	return &runtime.RawExtension{Raw: controlPlaneConfig}
}

func fixAzureInfrastructureConfig(workersCIDR string, zones []string) *runtime.RawExtension {
	infraConfig, _ := azure.GetInfrastructureConfig(workersCIDR, zones)
	return &runtime.RawExtension{Raw: infraConfig}
}

func fixMachineImage(machineImageName, machineImageVersion string) *gardener.ShootMachineImage {
	if machineImageVersion != "" {
		return &gardener.ShootMachineImage{
			Version: &machineImageVersion,
			Name:    machineImageName,
		}
	}

	return &gardener.ShootMachineImage{}
}

func TestGetAllWorkersZones(t *testing.T) {
	tests := []struct {
		name     string
		workers  []gardener.Worker
		expected []string
		wantErr  bool
	}{
		{
			name: "Single worker with zones",
			workers: []gardener.Worker{
				{
					Name:  "worker1",
					Zones: []string{"zone1", "zone2"},
				},
			},
			expected: []string{"zone1", "zone2"},
			wantErr:  false,
		},
		{
			name: "Multiple workers with same zones",
			workers: []gardener.Worker{
				{
					Name:  "worker1",
					Zones: []string{"zone1", "zone2"},
				},
				{
					Name:  "worker2",
					Zones: []string{"zone1", "zone2"},
				},
			},
			expected: []string{"zone1", "zone2"},
			wantErr:  false,
		},
		{
			name: "Multiple workers with different zones",
			workers: []gardener.Worker{
				{
					Name:  "worker1",
					Zones: []string{"zone1", "zone2"},
				},
				{
					Name:  "worker2",
					Zones: []string{"zone1"},
				},
			},
			expected: []string{"zone1", "zone2"},
			wantErr:  false,
		},
		{
			name:     "No workers provided",
			workers:  []gardener.Worker{},
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zones, err := getNetworkingZonesFromWorkers(tt.workers)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, zones)
			}
		})
	}
}

func assertProvider(t *testing.T, runtimeShoot imv1.RuntimeShoot, shoot gardener.Shoot, expectWorkerConfig bool, expectedMachineImageName, expectedMachineImageVersion string) {
	assert.Equal(t, runtimeShoot.Provider.Type, shoot.Spec.Provider.Type)
	assert.Equal(t, runtimeShoot.Provider.Workers, shoot.Spec.Provider.Workers)
	assert.Equal(t, false, shoot.Spec.Provider.WorkersSettings.SSHAccess.Enabled)
	assert.NotEmpty(t, shoot.Spec.Provider.InfrastructureConfig)
	assert.NotEmpty(t, shoot.Spec.Provider.InfrastructureConfig.Raw)
	assert.NotEmpty(t, shoot.Spec.Provider.ControlPlaneConfig)
	assert.NotEmpty(t, shoot.Spec.Provider.ControlPlaneConfig.Raw)

	for _, worker := range shoot.Spec.Provider.Workers {
		if expectWorkerConfig {
			assert.NotEmpty(t, worker.ProviderConfig)
			assert.NotEmpty(t, worker.ProviderConfig.Raw)
		} else {
			assert.Empty(t, worker.ProviderConfig)
		}
		assert.Equal(t, expectedMachineImageVersion, *worker.Machine.Image.Version)
		assert.Equal(t, expectedMachineImageName, worker.Machine.Image.Name)
	}
}

func assertProviderMultipleWorkers(t *testing.T, runtimeShoot imv1.RuntimeShoot, shoot gardener.Shoot, expectWorkerConfig bool, expectedWorkers []gardener.Worker) {
	assert.Equal(t, len(expectedWorkers), len(shoot.Spec.Provider.Workers))
	assert.Equal(t, shoot.Spec.Provider.Type, runtimeShoot.Provider.Type)
	assert.Equal(t, false, shoot.Spec.Provider.WorkersSettings.SSHAccess.Enabled)
	assert.NotEmpty(t, shoot.Spec.Provider.InfrastructureConfig)
	assert.NotEmpty(t, shoot.Spec.Provider.InfrastructureConfig.Raw)
	assert.NotEmpty(t, shoot.Spec.Provider.ControlPlaneConfig)
	assert.NotEmpty(t, shoot.Spec.Provider.ControlPlaneConfig.Raw)

	for i, worker := range shoot.Spec.Provider.Workers {
		if expectWorkerConfig {
			assert.NotEmpty(t, worker.ProviderConfig)
			assert.NotEmpty(t, worker.ProviderConfig.Raw)
		} else {
			assert.Empty(t, worker.ProviderConfig)
		}

		expected := expectedWorkers[i]
		assert.Equal(t, expected.Name, worker.Name)
		assert.Equal(t, expected.Zones, worker.Zones)
		assert.Equal(t, expected.Minimum, worker.Minimum)
		assert.Equal(t, expected.Maximum, worker.Maximum)
		assert.Equal(t, expected.Machine.Type, worker.Machine.Type)

		if expected.MaxSurge != nil {
			assert.NotNil(t, worker.MaxSurge)
			assert.Equal(t, *expected.MaxSurge, *worker.MaxSurge)
		}

		if expected.MaxUnavailable != nil {
			assert.NotNil(t, worker.MaxUnavailable)
			assert.Equal(t, *expected.MaxUnavailable, *worker.MaxUnavailable)
		}

		if expected.Machine.Architecture != nil {
			assert.NotNil(t, worker.Machine.Architecture)
			assert.Equal(t, *expected.Machine.Architecture, *worker.Machine.Architecture)
		}

		assert.NotNil(t, worker.Machine.Image)
		assert.Equal(t, expected.Machine.Image.Name, worker.Machine.Image.Name)
		assert.NotEmpty(t, expected.Machine.Image.Version)
		assert.NotEmpty(t, worker.Machine.Image.Version)
		assert.Equal(t, *expected.Machine.Image.Version, *worker.Machine.Image.Version)
	}
}

func assertProviderSpecificConfigAWS(t *testing.T, shoot gardener.Shoot, expectedZonesCount int) {
	var infrastructureConfig v1alpha1.InfrastructureConfig

	err := json.Unmarshal(shoot.Spec.Provider.InfrastructureConfig.Raw, &infrastructureConfig)
	require.NoError(t, err)

	assert.Equal(t, expectedZonesCount, len(infrastructureConfig.Networks.Zones))
}

func assertProviderSpecificConfigAzure(t *testing.T, shoot gardener.Shoot, expectedZonesCount int) {
	var infrastructureConfig azure.InfrastructureConfig

	err := json.Unmarshal(shoot.Spec.Provider.InfrastructureConfig.Raw, &infrastructureConfig)
	require.NoError(t, err)

	assert.Equal(t, expectedZonesCount, len(infrastructureConfig.Networks.Zones))
}

func assertProviderSpecificConfigGCP(t *testing.T, shoot gardener.Shoot, expectedZonesCount int) {
	var infrastructureConfig gcp.InfrastructureConfig

	err := json.Unmarshal(shoot.Spec.Provider.InfrastructureConfig.Raw, &infrastructureConfig)
	require.NoError(t, err)

	// validate the networking cidr here
}
