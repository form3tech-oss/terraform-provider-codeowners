package file

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/go-github/v54/github"
)

var (
	// ErrNotFound is the error returned by GetFile when the target file does not exist.
	ErrNotFound = errors.New("file not found")
)

// GetFile returns a handle to a file in a GitHub repository by branch and path.
// If an empty value is provided as the name of the branch, the file is looked up in the default one.
func GetFile(
	ctx context.Context,
	c *github.Client,
	repositoryOwner, repositoryName, branch, path string,
) (*github.RepositoryContent, error) {
	var o *github.RepositoryContentGetOptions
	if branch != "" {
		o = &github.RepositoryContentGetOptions{
			Ref: branch,
		}
	}
	f, _, res, err := c.Repositories.GetContents(ctx, repositoryOwner, repositoryName, path, o)
	if err != nil {
		if res == nil {
			return nil, fmt.Errorf("failed to read %q: %v", path, err)
		}
		if res.StatusCode == http.StatusNotFound {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to read %q (status code: %d): %v", path, res.StatusCode, err)
	}
	return f, nil
}
