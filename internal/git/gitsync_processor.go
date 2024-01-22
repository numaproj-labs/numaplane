package git

import (
	"context"
	"errors"
	"io"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing/format/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/numaproj-labs/numaplane/api/v1"
	"github.com/numaproj-labs/numaplane/internal/shared/logging"
)

const (
	messageChanLength = 5
	timeInterval      = 3
)

type Message struct {
	Updates bool
	Err     error
}

var commitSHARegex = regexp.MustCompile("^[0-9A-Fa-f]{40}$")

// isCommitSHA returns whether or not a string is a 40 character SHA-1
func isCommitSHA(sha string) bool {
	return commitSHARegex.MatchString(sha)
}

type GitSyncProcessor struct {
	gitSync     v1.GitSync
	channels    map[string]chan Message
	k8Client    client.Client
	clusterName string
}

// stores the Patched files  and their data

type PatchFiles struct {
	DeletedFiles  map[string][]byte
	AddedFiles    map[string][]byte
	ModifiedFiles map[string][]byte
}

func (processor *GitSyncProcessor) watchRepo(ctx context.Context, repo *v1.RepositoryPath, _ /* namespace */ string) error {
	logger := logging.FromContext(ctx)

	r, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL:          repo.RepoUrl,
		SingleBranch: true,
	})
	if err != nil {
		logger.Errorw("error cloning the repository", "err", err)
		return err
	}

	// The revision can be a branch, a tag, or a commit hash
	h, err := r.ResolveRevision(plumbing.Revision(repo.TargetRevision))
	if err != nil {
		logger.Errorw("error resolving the revision", "revision", repo.TargetRevision, "err", err)
		return err
	}
	// save the commit hash in the gitSync status

	// Retrieving the commit object matching the hash.
	commit, err := r.CommitObject(*h)
	if err != nil {
		logger.Errorw("error checkout the commit", "hash", h.String(), "err", err)
		return err
	}

	// Retrieve the tree from the commit.
	tree, err := commit.Tree()
	if err != nil {
		logger.Errorw("error get the commit tree", "err", err)
		return err
	}

	// Locate the tree with the given path.
	tree, err = tree.Tree(repo.Path)
	if err != nil {
		logger.Errorw("error locate the path", "err", err)
		return err
	}

	// Read all the files under the path and apply each one respectively.
	err = tree.Files().ForEach(func(f *object.File) error {
		logger.Debugw("read file", "file_name", f.Name)
		_, err = f.Contents()
		if err != nil {
			logger.Errorw("cannot get file content", "filename", f.Name, "err", err)
			return err
		}

		// TODO: Apply to the resources
		return nil
	})
	if err != nil {
		return err
	}

	if isCommitSHA(repo.TargetRevision) {
		// TODO: no monitoring
		logger.Debug("no monitoring")

	} else {
		ticker := time.NewTicker(timeInterval * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				changedFiles, err := CheckForRepoUpdates(r, repo, &processor.gitSync.Status, ctx)
				if err != nil {
					return err
				}
				// apply the resources which are updated
				for _, v := range changedFiles.ModifiedFiles {
					// split the string on the basis of "---"
					individualResource := strings.Split(string(v), "---")
					logger.Debug(individualResource)
				}

				// delete the resources which are removed from file

				// create the new resource

			case <-ctx.Done():
				logger.Debug("Context canceled, stopping updates check")
				return nil
			}
		}
	}
	return nil
}

// this check for file changes in the repo by comparing the old commit  hash with new commit hash

func CheckForRepoUpdates(r *git.Repository, repo *v1.RepositoryPath, status *v1.GitSyncStatus, ctx context.Context) (PatchFiles, error) {
	var patchedFiles PatchFiles
	logger := logging.FromContext(ctx)
	if err := fetchUpdates(r); err != nil {
		logger.Errorw("error checking for updates in the github repo", "err", err)
		return patchedFiles, err
	}
	remoteRef, err := getLatestCommit(r, repo.TargetRevision)
	if err != nil {
		logger.Errorw("failed to get latest commits in the github repo", "err", err)
		return patchedFiles, err
	}
	lastCommitStatus := status.CommitStatus[repo.Name]
	log.Println("remote hash", remoteRef.String(), "lastCommit", status.CommitStatus)

	if remoteRef.String() != lastCommitStatus.Hash {
		logger.Debug("New changes detected. Comparing changes...")
		status.CommitStatus[repo.Name] = v1.CommitStatus{
			Hash:     remoteRef.String(),
			Synced:   true,
			SyncTime: metav1.Time{Time: time.Now()},
			Error:    "",
		}

		lastCommit, err := r.CommitObject(plumbing.NewHash(lastCommitStatus.Hash))
		if err != nil {
			logger.Errorw("error checkout the commit", "hash", lastCommitStatus.Hash, "err", err)
			return patchedFiles, err
		}

		lastTree, err := lastCommit.Tree()
		if err != nil {
			logger.Errorw("error getting the last tree", "hash", lastCommitStatus.Hash, "err", err)
			return patchedFiles, err
		}

		lastTreeForthePath, err := lastTree.Tree(repo.Path)
		if err != nil {
			logger.Errorw("error locate the path", "err", err)
			return patchedFiles, err
		}

		recentCommit, err := r.CommitObject(*remoteRef)
		if err != nil {
			logger.Errorw("failed to get commit object repo", "err", err)
			return patchedFiles, err
		}

		recentTree, err := recentCommit.Tree()
		if err != nil {
			logger.Errorw("error getting the last tree", "hash", remoteRef.String(), "err", err)
			return patchedFiles, err
		}

		recentTreeForThePath, err := recentTree.Tree(repo.Path)
		if err != nil {
			logger.Errorw("error locate the path", "err", err)
			return patchedFiles, err
		}

		patch, err := lastTreeForthePath.Patch(recentTreeForThePath)
		if err != nil {
			logger.Errorw("failed to patch commit", "err", err)
			return patchedFiles, err
		}

		deletedFiles := make(map[string][]byte)
		addedFiles := make(map[string][]byte)
		modifiedFiles := make(map[string][]byte)
		//if file exists in both [from] and [to] its modified ,if it exists in [to] its newly added and if it only exists in [from] its deleted
		for _, filePatch := range patch.FilePatches() {
			from, to := filePatch.Files()
			if from != nil && to != nil {
				contents, err := getBlobFileContents(r, to)
				if err != nil {
					return patchedFiles, err
				}
				modifiedFiles[to.Path()] = contents

			} else if from != nil {
				contents, err := getBlobFileContents(r, from)
				if err != nil {
					return patchedFiles, err
				}
				deletedFiles[from.Path()] = contents
			} else if to != nil {
				contents, err := getBlobFileContents(r, from)
				if err != nil {
					return patchedFiles, err
				}
				addedFiles[to.Path()] = contents
			}
		}

		patchedFiles.ModifiedFiles = modifiedFiles
		patchedFiles.AddedFiles = addedFiles
		patchedFiles.DeletedFiles = deletedFiles
	}

	return patchedFiles, nil
}

// gets the file content from the repository with file hash
func getBlobFileContents(r *git.Repository, file diff.File) ([]byte, error) {
	fileBlob, err := r.BlobObject(file.Hash())
	if err != nil {
		return nil, err
	}
	reader, err := fileBlob.Reader()
	if err != nil {
		return nil, err
	}
	fileContent, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return fileContent, nil
}

func fetchUpdates(repo *git.Repository) error {
	err := repo.Fetch(&git.FetchOptions{
		RemoteName: "origin",
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return err
	}
	return nil
}

// getLatestCommit retrieves the latest commit hash of a given branch or tag
func getLatestCommit(repo *git.Repository, refName string) (*plumbing.Hash, error) {
	commitHash, err := repo.ResolveRevision(plumbing.Revision(refName))
	if err != nil {
		return nil, err
	}
	return commitHash, err
}

func NewGitSyncProcessor(ctx context.Context, gitSync *v1.GitSync, k8client client.Client, clusterName string) (*GitSyncProcessor, error) {
	logger := logging.FromContext(ctx)
	channels := make(map[string]chan Message)
	namespace := gitSync.Spec.GetDestinationNamespace(clusterName)
	processor := &GitSyncProcessor{
		gitSync:     *gitSync,
		k8Client:    k8client,
		channels:    channels,
		clusterName: clusterName,
	}
	for _, repo := range gitSync.Spec.RepositoryPaths {
		gitCh := make(chan Message, messageChanLength)
		channels[repo.Name] = gitCh
		go func(repo *v1.RepositoryPath) {
			err := processor.watchRepo(ctx, repo, namespace)
			if err != nil {
				// TODO: Retry on non-fatal errors
				logger.Errorw("error watching the repo", "err", err)
			}
		}(&repo)
	}

	return processor, nil
}

func (processor *GitSyncProcessor) Update(gitSync *v1.GitSync) error {
	return nil
}

func (processor *GitSyncProcessor) Shutdown() error {
	return nil
}
