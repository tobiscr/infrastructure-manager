package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardener_types "github.com/gardener/gardener/pkg/client/core/clientset/versioned/typed/core/v1beta1"
	migrator "github.com/kyma-project/infrastructure-manager/hack/runtime-migrator-app/internal"
	"github.com/kyma-project/infrastructure-manager/hack/runtime-migrator-app/internal/runtime"
	"github.com/kyma-project/infrastructure-manager/pkg/gardener"
	"github.com/kyma-project/infrastructure-manager/pkg/gardener/kubeconfig"
	"github.com/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
	"log/slog"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

const (
	migratorLabel                      = "operator.kyma-project.io/created-by-migrator"
	expirationTime                     = 60 * time.Minute
	ShootNetworkingFilterExtensionType = "shoot-networking-filter"
	runtimeCrFullPath                  = "%sshoot-%s.yaml"
	runtimeIDAnnotation                = "kcp.provisioner.kyma-project.io/runtime-id"
	contextTimeout                     = 5 * time.Minute
)

func main() {
	slog.Info("Starting runtime-migrator")
	cfg := migrator.NewConfig()
	_, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()

	converterConfig, err := migrator.LoadConverterConfig(cfg)
	if err != nil {
		slog.Error(fmt.Sprintf("Unable to load converter config - %v", err))
		os.Exit(1)
	}

	gardenerNamespace := fmt.Sprintf("garden-%s", cfg.GardenerProjectName)

	kubeconfigProvider, err := setupKubernetesKubeconfigProvider(cfg.GardenerKubeconfigPath, gardenerNamespace, expirationTime)
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to create kubeconfig provider - %v", err))
		os.Exit(1)
	}

	kcpClient, err := migrator.CreateKcpClient(&cfg)
	if err != nil {
		slog.Error("failed to create kcp client - ", kcpClient)
		os.Exit(1)
	}

	gardenerShootClient, err := setupGardenerShootClient(cfg.GardenerKubeconfigPath, gardenerNamespace)
	runtimeMigrator := runtime.NewMigrator(converterConfig, kubeconfigProvider, kcpClient)

	slog.Info("Migrating runtimes")
	stats, err := migrateRuntimes(runtimeMigrator, gardenerShootClient, getRuntimeIDsFromStdin(cfg))
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to migrate runtimes - %v", err))
		os.Exit(1)
	}

	slog.Info(fmt.Sprintf("Migration completed. Successfully migrated runtimes: %d, Failed migrations: %d", stats.Success, stats.Failure))
}

type stats struct {
	Success int
	Failure int
}

func migrateRuntimes(runtimeMigrator runtime.Migrator, shootClient gardener_types.ShootInterface, runtimeIDs []string) (stats, error) {

	successfulMigrations := 0
	failedMigrations := 0

	shootList, err := shootClient.List(context.Background(), v1.ListOptions{})
	if err != nil {
		return stats{}, err
	}

	findShoot := func(runtimeID string) *v1beta1.Shoot {
		for _, shoot := range shootList.Items {
			if shoot.Annotations[runtimeIDAnnotation] == runtimeID {
				return &shoot
			}
		}
		return nil
	}

	for _, runtimeID := range runtimeIDs {
		slog.Info(fmt.Sprintf("Migrating runtime with ID: %s", runtimeID))
		shoot := findShoot(runtimeID)
		if shoot == nil {
			slog.Error(fmt.Sprintf("Failed to find shoot for runtime with ID: %s", runtimeID))
			failedMigrations++
			continue
		}

		_, err := runtimeMigrator.Do(*shoot)
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to migrate runtime with ID: %s - %v", runtimeID, err))
			failedMigrations++
			continue
		}
	}

	return stats{
		Success: successfulMigrations,
		Failure: failedMigrations,
	}, nil
}

func setupKubernetesKubeconfigProvider(kubeconfigPath string, namespace string, expirationTime time.Duration) (kubeconfig.Provider, error) {
	restConfig, err := gardener.NewRestConfigFromFile(kubeconfigPath)
	if err != nil {
		return kubeconfig.Provider{}, err
	}

	gardenerClientSet, err := gardener_types.NewForConfig(restConfig)
	if err != nil {
		return kubeconfig.Provider{}, err
	}

	gardenerClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return kubeconfig.Provider{}, err
	}

	shootClient := gardenerClientSet.Shoots(namespace)
	dynamicKubeconfigAPI := gardenerClient.SubResource("adminkubeconfig")

	err = v1beta1.AddToScheme(gardenerClient.Scheme())
	if err != nil {
		return kubeconfig.Provider{}, errors.Wrap(err, "failed to register Gardener schema")
	}

	return kubeconfig.NewKubeconfigProvider(shootClient,
		dynamicKubeconfigAPI,
		namespace,
		int64(expirationTime.Seconds())), nil
}

func getRuntimeIDsFromStdin(cfg migrator.Config) []string {
	var runtimeIDs []string
	if cfg.InputType == migrator.InputTypeJSON {
		decoder := json.NewDecoder(os.Stdin)
		slog.Info("Reading runtimeIds from stdin")
		if err := decoder.Decode(&runtimeIDs); err != nil {
			log.Printf("Could not load list of RuntimeIds - %s", err)
		}
	}
	return runtimeIDs
}

func setupGardenerShootClient(kubeconfigPath, gardenerNamespace string) (gardener_types.ShootInterface, error) {
	restConfig, err := gardener.NewRestConfigFromFile(kubeconfigPath)
	if err != nil {
		return nil, err
	}

	gardenerClientSet, err := gardener_types.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	shootClient := gardenerClientSet.Shoots(gardenerNamespace)

	return shootClient, nil
}
