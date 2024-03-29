package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"

	argoGit "github.com/argoproj/argo-cd/v2/util/git"
	"github.com/argoproj/argo-cd/v2/util/helm"
	pathutil "github.com/argoproj/argo-cd/v2/util/io/path"
	"github.com/argoproj/argo-cd/v2/util/kustomize"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/cespare/xxhash/v2"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	controllerConfig "github.com/numaproj-labs/numaplane/internal/controller/config"
	gitShared "github.com/numaproj-labs/numaplane/internal/util/git"
	"github.com/numaproj-labs/numaplane/internal/util/kubernetes"
	"github.com/numaproj-labs/numaplane/internal/util/logging"
	"github.com/numaproj-labs/numaplane/pkg/apis/numaplane/v1alpha1"
)

func CloneRepo(
	ctx context.Context,
	client k8sClient.Client,
	gitSync *v1alpha1.GitSync,
	globalConfig controllerConfig.GlobalConfig,
) (*git.Repository, error) {
	gitCredentials := gitShared.FindCredByUrl(gitSync.Spec.RepoUrl, globalConfig)
	cloneOptions, err := gitShared.GetRepoCloneOptions(ctx, gitCredentials, client, gitSync.Spec.RepoUrl)
	if err != nil {
		return nil, fmt.Errorf("error getting  the  clone options: %v", err)
	}
	return cloneRepo(ctx, gitSync, cloneOptions)
}

// GetLatestManifests gets the latest manifests from the Git repository.
// TODO: fetches updates from the remote repository, checks the latest
//
//	commit hash against the stored hash in the GitSync object.
func GetLatestManifests(
	ctx context.Context,
	r *git.Repository,
	client k8sClient.Client,
	gitSync *v1alpha1.GitSync,
) (string, []*unstructured.Unstructured, error) {
	logger := logging.FromContext(ctx).With("GitSync name", gitSync.Name, "repo", gitSync.Spec.RepoUrl)

	// Fetch all remote branches
	err := fetchUpdates(ctx, client, gitSync, r)
	if err != nil {
		return "", nil, err
	}

	// The revision can be a branch, a tag, or a commit hash
	hash, err := getLatestCommitHash(r, gitSync.Spec.TargetRevision)
	if err != nil {
		logger.Errorw("error resolving the revision", "revision", gitSync.Spec.TargetRevision, "err", err, "repo", gitSync.Spec.RepoUrl)
		return "", nil, err
	}

	localRepoPath := getLocalRepoPath(gitSync)

	// deployablePath will be the path of cloned repository + the path while needs to be deployed.
	deployablePath := localRepoPath + "/" + gitSync.Spec.Path
	// validate the existence of path.
	if _, err := os.Stat(deployablePath); err != nil {
		return "", nil, fmt.Errorf("invalid repo path, err: %v", err)
	}

	sourceType, err := gitSync.Spec.ExplicitType()
	if err != nil {
		return "", nil, err
	}

	var targetObjs []*unstructured.Unstructured
	switch sourceType {
	case v1alpha1.SourceTypeKustomize:
		k := kustomize.NewKustomizeApp(localRepoPath, gitSync.Spec.Path, argoGit.NopCreds{}, gitSync.Spec.RepoUrl, os.Getenv("KUSTOMIZE_BINARY_PATH"))
		targetObjs, _, err = k.Build(nil, nil, nil)
	case v1alpha1.SourceTypeHelm:
		targetObjs, err = helmTemplate(localRepoPath, gitSync)
	case v1alpha1.SourceTypeRaw:
		targetObjs, err = rawTemplate(r, gitSync, hash)
	}
	if err != nil {
		return "", nil, fmt.Errorf("failed to build manifests, err: %v", err)
	}

	return hash.String(), targetObjs, nil
}

// helmTemplate will return the list of unstructured objects after templating the helm chart.
func helmTemplate(localRepoPath string, gitSync *v1alpha1.GitSync) ([]*unstructured.Unstructured, error) {
	h, err := helm.NewHelmApp(localRepoPath+"/"+gitSync.Spec.Path, []helm.HelmRepository{{Repo: gitSync.Spec.RepoUrl}},
		false, "", "", false)
	if err != nil {
		return nil, fmt.Errorf("error initializing helm app object: %w", err)
	}

	templateOpts := &helm.TemplateOpts{
		Name:      gitSync.Name,
		Namespace: gitSync.Spec.Destination.Namespace,
		Set:       map[string]string{},
		SkipCrds:  true,
	}

	for _, value := range gitSync.Spec.Helm.ValueFiles {
		templateOpts.Values = append(templateOpts.Values, pathutil.ResolvedFilePath(value))
	}

	for _, p := range gitSync.Spec.Helm.Parameters {
		templateOpts.Set[p.Name] = p.Value
	}

	out, err := h.Template(templateOpts)
	if err != nil {
		if !helm.IsMissingDependencyErr(err) {
			return nil, err
		}
		if depErr := h.DependencyBuild(); depErr != nil {
			return nil, depErr
		}

		if out, err = h.Template(templateOpts); err != nil {
			return nil, err
		}
	}

	return kube.SplitYAML([]byte(out))
}

// rawTemplate will return the list of unstructured objects after templating the raw manifest/yaml files.
func rawTemplate(r *git.Repository, gitSync *v1alpha1.GitSync, hash *plumbing.Hash) ([]*unstructured.Unstructured, error) {
	var manifestObj []*unstructured.Unstructured
	tree, err := getCommitTreeAtPath(r, gitSync.Spec.Path, *hash)
	if err != nil {
		return nil, err
	}
	// Read all the files under the path and apply each one respectively.
	if err = tree.Files().ForEach(func(f *object.File) error {
		if kubernetes.IsValidKubernetesManifestFile(f.Name) {
			manifest, err := f.Contents()
			if err != nil {
				return fmt.Errorf("failed to get file content, filename: %s, err: %v", f.Name, err)
			}
			manifestData, err := kube.SplitYAML([]byte(manifest))
			if err != nil {
				return fmt.Errorf("can not parse file data, err: %v", err)
			}
			manifestObj = append(manifestObj, manifestData...)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return manifestObj, nil
}

// getLatestCommitHash retrieves the latest commit hash of a given branch or tag
func getLatestCommitHash(repo *git.Repository, refName string) (*plumbing.Hash, error) {
	commitHash, err := repo.ResolveRevision(plumbing.Revision(refName))
	if err != nil {
		return nil, err
	}
	return commitHash, err
}

// getCommitTreeAtPath retrieves a specific tree (or subtree) located at a given path within a specific commit in a Git repository
func getCommitTreeAtPath(r *git.Repository, path string, hash plumbing.Hash) (*object.Tree, error) {
	commit, err := r.CommitObject(hash)
	if err != nil {
		return nil, err
	}
	commitTree, err := commit.Tree()
	if err != nil {
		return nil, err
	}
	if !isRootDir(path) {
		commitTreeForPath, err := commitTree.Tree(path)
		if err != nil {
			return nil, err
		}
		return commitTreeForPath, nil
	}
	return commitTree, nil
}

// isRootDir returns whether this given path represents root directory of a repo,
// We consider empty string as the root.
func isRootDir(path string) bool {
	return len(path) == 0
}

// getLocalRepoPath will return the local path where repo will be cloned,
// by default it will use /tmp as base directory
// unless LOCAL_REPO_PATH env is set.
func getLocalRepoPath(gitSync *v1alpha1.GitSync) string {
	baseDir := os.Getenv("LOCAL_REPO_PATH")
	repoUrl := strconv.FormatUint(xxhash.Sum64([]byte(gitSync.Spec.RepoUrl)), 16)
	if baseDir != "" {
		return fmt.Sprintf("%s/%s/%s", baseDir, gitSync.Name, repoUrl)
	} else {
		return fmt.Sprintf("/tmp/%s/%s", gitSync.Name, repoUrl)
	}
}

// fetchUpdates fetches all the remote branches and updates the local changes, returning nil if already up-to-date or an error otherwise.
func fetchUpdates(ctx context.Context,
	client k8sClient.Client,
	gitSync *v1alpha1.GitSync, repo *git.Repository) error {

	remote, err := repo.Remote("origin")
	if err != nil {
		return err
	}

	globalConfig, err := controllerConfig.GetConfigManagerInstance().GetConfig()
	if err != nil {
		return err
	}

	credentials := gitShared.FindCredByUrl(gitSync.Spec.RepoUrl, globalConfig)

	fetchOptions, err := gitShared.GetRepoFetchOptions(ctx, credentials, client, gitSync.Spec.RepoUrl)
	if err != nil {
		return err
	}

	err = remote.Fetch(fetchOptions)
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return err
	}

	return nil
}

func cloneRepo(ctx context.Context, gitSync *v1alpha1.GitSync, options *git.CloneOptions) (*git.Repository, error) {
	path := getLocalRepoPath(gitSync)

	r, err := git.PlainCloneContext(ctx, path, false, options)
	if err != nil && errors.Is(err, git.ErrRepositoryAlreadyExists) {
		// open the existing repo and return it.
		existingRepo, openErr := git.PlainOpen(path)
		if openErr != nil {
			return r, fmt.Errorf("failed to open existing repo, err: %v", openErr)
		}
		return existingRepo, nil
	}

	return r, err
}
