package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/storer"

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
	"github.com/numaproj-labs/numaplane/internal/metrics"
	gitShared "github.com/numaproj-labs/numaplane/internal/util/git"
	"github.com/numaproj-labs/numaplane/internal/util/kubernetes"
	"github.com/numaproj-labs/numaplane/internal/util/logger"
	"github.com/numaproj-labs/numaplane/pkg/apis/numaplane/v1alpha1"
)

func CloneRepo(
	ctx context.Context,
	client k8sClient.Client,
	gitSync *v1alpha1.GitSync,
	globalConfig controllerConfig.GlobalConfig,
	metricServer *metrics.MetricsServer,
) (*git.Repository, error) {
	numaLogger := logger.FromContext(ctx).WithValues("gitSyncName", gitSync.Name, "repo", gitSync.Spec.RepoUrl)
	gitCredentials := gitShared.FindCredByUrl(gitSync.Spec.RepoUrl, globalConfig)
	numaLogger.Debugf("git credentials associated with URL=%+v", gitCredentials)

	cloneOptions, err := gitShared.GetRepoCloneOptions(ctx, gitCredentials, client, gitSync.Spec.RepoUrl)
	if err != nil {
		return nil, fmt.Errorf("error getting the clone options: %v", err)
	}

	gitConfig, err := config.LoadConfig(config.GlobalScope)
	if err != nil {
		return nil, fmt.Errorf("error loading git config: %v", err)
	}

	err = gitShared.UpdateOptionsWithGitConfig(gitConfig, cloneOptions)
	if err != nil {
		return nil, fmt.Errorf("error updating clone options with git config: %v", err)
	}

	return cloneRepo(ctx, gitSync, cloneOptions, metricServer)
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
	metricServer *metrics.MetricsServer,
) (string, []*unstructured.Unstructured, error) {
	numaLogger := logger.FromContext(ctx).WithValues("GitSync name", gitSync.Name, "repo", gitSync.Spec.RepoUrl)

	err := fetchUpdates(ctx, client, gitSync, r, metricServer)
	if err != nil {
		return "", nil, err
	}

	// The revision can be a branch, a tag, or a commit hash
	hash, err := getLatestCommitHash(r, gitSync.Spec.TargetRevision)
	if err != nil {
		numaLogger.Error(err, "error resolving the revision", "revision", gitSync.Spec.TargetRevision, "repo", gitSync.Spec.RepoUrl)
		return "", nil, err
	}

	localRepoPath, err := getLocalRepoPath(gitSync)
	if err != nil {
		return "", nil, err
	}

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
	return commitHash, nil
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
func getLocalRepoPath(gitSync *v1alpha1.GitSync) (string, error) {
	// baseDir is persistent volume path on the cluster node
	baseDir := "/tmp"
	globalConfig, err := controllerConfig.GetConfigManagerInstance().GetConfig()
	if err != nil {
		return "", fmt.Errorf("error getting config manager for repoLocalPath %s", err)
	}
	if globalConfig.PersistentRepoClonePath != "" {
		baseDir = globalConfig.PersistentRepoClonePath
	}
	repoUrl := strconv.FormatUint(xxhash.Sum64([]byte(gitSync.Spec.RepoUrl)), 16)
	return fmt.Sprintf("%s/%s/%s", baseDir, gitSync.Name, repoUrl), nil
}

func GetCurrentBranch(r *git.Repository) (string, error) {
	// Get the HEAD reference to determine the current branch.
	ref, err := r.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	// Check if the HEAD is a symbolic reference (which means it is pointing to a branch).
	if ref.Name().IsBranch() {
		return ref.Name().Short(), nil
	}

	// If HEAD is not a symbolic reference, it might be a detached HEAD.
	return "", fmt.Errorf("HEAD is detached and not pointing to any branch")
}

// fetchUpdates fetches the remote branch and updates the local changes, returning nil if already up-to-date or an error otherwise.
func fetchUpdates(ctx context.Context,
	client k8sClient.Client,
	gitSync *v1alpha1.GitSync,
	repo *git.Repository,
	metricServer *metrics.MetricsServer,
) error {
	globalConfig, err := controllerConfig.GetConfigManagerInstance().GetConfig()
	if err != nil {
		return err
	}

	credentials := gitShared.FindCredByUrl(gitSync.Spec.RepoUrl, globalConfig)

	branch, err := GetCurrentBranch(repo)
	// Only fetch all and checkout to the new branch if there is an
	// error or the branch has changed.
	if err != nil || branch != gitSync.Spec.TargetRevision {
		fetchOptions, err := gitShared.GetRepoFetchOptions(ctx, credentials, client, gitSync.Spec.RepoUrl)
		if err != nil {
			return fmt.Errorf("error getting the fetch options: %v", err)
		}

		// No need to fetch the references if the target revision is already main or master (branches)
		if gitSync.Spec.TargetRevision != "main" && gitSync.Spec.TargetRevision != "master" {
			// fetch all references
			err = fetchAll(repo, fetchOptions)
			if err != nil {
				return err
			}
		}

		// Perform checkout to the specified reference after a successful clone
		if checkoutErr := checkoutRepo(repo, gitSync.Spec.TargetRevision); checkoutErr != nil {
			return checkoutErr
		}

		branch, err = GetCurrentBranch(repo)
		if err != nil {
			return err
		}
	}

	pullOptions, err := gitShared.GetRepoPullOptions(ctx, credentials, client, gitSync.Spec.RepoUrl, branch)
	if err != nil {
		return err
	}

	gitConfig, err := config.LoadConfig(config.GlobalScope)
	if err != nil {
		return fmt.Errorf("error loading git config: %v", err)
	}

	err = gitShared.UpdateOptionsWithGitConfig(gitConfig, pullOptions)
	if err != nil {
		return fmt.Errorf("error updating pull options with git config: %v", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return err
	}

	startTime := time.Now()
	defer func() {
		metricServer.IncGitRequest()
		metricServer.ObserveGitRequestLatency(time.Since(startTime))
	}()
	if err = worktree.Pull(pullOptions); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		metricServer.IncGitRequestFailed()
		return err
	}

	return nil
}

func cloneRepo(
	ctx context.Context,
	gitSync *v1alpha1.GitSync,
	cloneOptions *git.CloneOptions,
	metricServer *metrics.MetricsServer,
) (*git.Repository, error) {
	path, err := getLocalRepoPath(gitSync)
	if err != nil {
		return nil, err
	}
	defer func() {
		metricServer.IncGitRequest()
	}()

	r, err := git.PlainCloneContext(ctx, path, false, cloneOptions)
	if err != nil {
		if errors.Is(err, git.ErrRepositoryAlreadyExists) {
			// Open the existing repo and return it.
			existingRepo, openErr := git.PlainOpen(path)
			if openErr != nil {
				return nil, fmt.Errorf("failed to open existing repo, err: %v", openErr)
			}
			return existingRepo, nil
		}
		return nil, err
	}

	return r, nil
}

func findFirstBranchContainingTag(repo *git.Repository, tagName string) (string, error) {
	// Find the tag and resolve it to a commit
	tagRef, err := repo.Tag(tagName)
	if err != nil {
		return "", fmt.Errorf("failed to find tag '%s': %v", tagName, err)
	}
	commit, err := repo.CommitObject(tagRef.Hash())
	if err != nil {
		return "", fmt.Errorf("failed to get commit from tag '%s': %v", tagName, err)
	}

	// List all branches and check if they contain the commit
	refs, err := repo.Branches()
	if err != nil {
		return "", fmt.Errorf("failed to list branches: %v", err)
	}

	var firstBranchContainingTag string
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		// Check each branch if it contains the commit
		commitIter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
		if err != nil {
			return err
		}
		err = commitIter.ForEach(func(c *object.Commit) error {
			if c.Hash == commit.Hash {
				firstBranchContainingTag = ref.Name().Short()
				return storer.ErrStop
			}
			return nil
		})

		if err != nil {
			return err
		}
		if firstBranchContainingTag != "" {
			return storer.ErrStop
		}
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("error while checking branches: %v", err)
	}
	if firstBranchContainingTag == "" {
		return "", fmt.Errorf("no branch found containing the commit from tag '%s'", tagName)
	}

	return firstBranchContainingTag, nil
}

func checkoutRepo(repo *git.Repository, refName string) error {
	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %v", err)
	}

	// Try to resolve as branch first
	branchRef, err := repo.Reference(plumbing.ReferenceName("refs/heads/"+refName), true)
	if err == nil {
		return w.Checkout(&git.CheckoutOptions{
			Branch: branchRef.Name(),
		})
	}

	// Try as a tag next
	firstBranch, err := findFirstBranchContainingTag(repo, refName)
	if err == nil {
		return w.Checkout(&git.CheckoutOptions{
			Branch: plumbing.ReferenceName("refs/heads/" + firstBranch),
		})
	}

	// Try as a commit hash
	firstBranchFromCommit, err := findFirstBranchContainingCommit(repo, refName)
	if err == nil {
		return w.Checkout(&git.CheckoutOptions{
			Branch: plumbing.ReferenceName("refs/heads/" + firstBranchFromCommit),
		})
	}

	return fmt.Errorf("reference '%s' not found as a branch, tag, or in any branch as a commit: %v", refName, err)
}

func fetchAll(
	repo *git.Repository,
	fetchOptions *git.FetchOptions,
) error {
	remote, err := repo.Remote("origin")
	if err != nil {
		return fmt.Errorf("failed to get remote: %v", err)
	}

	err = remote.Fetch(fetchOptions)
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return fmt.Errorf("failed to fetch repo: %v", err)
	}
	return nil
}

func findFirstBranchContainingCommit(repo *git.Repository, commitHash string) (string, error) {
	// Convert the string hash to a proper Hash object
	hash := plumbing.NewHash(commitHash)

	// List all branches
	refs, err := repo.Branches()
	if err != nil {
		return "", fmt.Errorf("failed to list branches: %v", err)
	}

	// Variable to store the name of the first branch containing the commit
	var firstBranchContainingCommit string

	// Iterate over all branches to check whether the commit hash belongs to any branch
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		// Check if the branch contains the commit
		commitIter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
		if err != nil {
			return err
		}
		err = commitIter.ForEach(func(c *object.Commit) error {
			if c.Hash == hash {
				firstBranchContainingCommit = ref.Name().Short()
				return storer.ErrStop
			}
			return nil
		})
		if err != nil {
			return err
		}
		if firstBranchContainingCommit != "" {
			return storer.ErrStop
		}
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("error while checking branches: %v", err)
	}
	if firstBranchContainingCommit == "" {
		return "", fmt.Errorf("no branch found containing the commit %s", commitHash)
	}

	return firstBranchContainingCommit, nil
}
