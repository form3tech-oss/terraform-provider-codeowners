package branch

import (
	"context"

	"github.com/google/go-github/v54/github"
)

// GetDefaultBranch returns the name of the default branch for the specified repository.
func GetDefaultBranch(ctx context.Context, c *github.Client, repositoryOwner, repositoryName string) (string, error) {
	r, _, err := c.Repositories.Get(ctx, repositoryOwner, repositoryName)
	if err != nil {
		return "", err
	}
	return *r.DefaultBranch, err
}

// GetSHAForBranch returns the SHA1 of the specified branch (or of the default one) of the specified repository.
func GetSHAForBranch(
	ctx context.Context,
	c *github.Client,
	repositoryOwner, repositoryName, branch string,
) (string, error) {
	if branch == "" {
		b, err := GetDefaultBranch(ctx, c, repositoryOwner, repositoryName)
		if err != nil {
			return "", err
		}
		branch = b
	}
	ref, _, err := c.Git.GetRef(ctx, repositoryOwner, repositoryName, "refs/heads/"+branch)
	if err != nil {
		return "", err
	}
	return *ref.Object.SHA, nil
}
