package watcher

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	controllerConfig "github.com/numaproj-labs/numaplane/internal/controller/config"
	"github.com/numaproj-labs/numaplane/internal/git"
	"github.com/numaproj-labs/numaplane/internal/metrics"
	"github.com/numaproj-labs/numaplane/internal/util/logger"
	"github.com/numaproj-labs/numaplane/pkg/apis/numaplane/v1alpha1"
)

func CheckForChanges(ctx context.Context, client client.Client, gitSync *v1alpha1.GitSync, globalConfig controllerConfig.GlobalConfig, metricsServer *metrics.MetricsServer, namespacedName types.NamespacedName) (bool, string, []*unstructured.Unstructured, error) {
	numaLogger := logger.FromContext(ctx).WithName("checkForChanges")

	// Clone the repository and get the latest manifests
	repo, err := git.CloneRepo(ctx, client, gitSync, globalConfig, metricsServer)
	if err != nil {
		return false, "", nil, fmt.Errorf("failed to clone the repo: %w", err)
	}
	commitHash, manifests, err := git.GetLatestManifests(ctx, repo, client, gitSync, metricsServer)
	if err != nil {
		return false, "", nil, fmt.Errorf("failed to fetch the latest manifests: %w", err)
	}

	// Get the last synced commit hash
	lastSyncedCommitHash, err := getCommitStatus(ctx, client, namespacedName, numaLogger)
	if err != nil {
		return false, "", nil, fmt.Errorf("failed to get the last commit status: %w", err)
	}

	// Compare the last synced hash with the current commit hash
	hasChanges := lastSyncedCommitHash == nil || lastSyncedCommitHash.Hash != commitHash
	return hasChanges, commitHash, manifests, nil
}
func getCommitStatus(
	ctx context.Context,
	kubeClient client.Client,
	namespacedName types.NamespacedName,
	numaLogger *logger.NumaLogger,
) (*v1alpha1.CommitStatus, error) {
	gitSync := &v1alpha1.GitSync{}
	if err := kubeClient.Get(ctx, namespacedName, gitSync); err != nil {
		// if we aren't able to do a Get, then either it's been deleted in the past, or something else went wrong
		if apierrors.IsNotFound(err) {
			return nil, nil
		} else {
			numaLogger.Error(err, "Unable to get GitSync", "err")
			return nil, err
		}
	}
	return gitSync.Status.CommitStatus, nil
}
