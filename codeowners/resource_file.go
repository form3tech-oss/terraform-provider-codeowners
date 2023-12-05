package codeowners

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/go-github/v54/github"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"

	"github.com/form3tech-oss/go-github-utils/pkg/branch"
	githubcommitutils "github.com/form3tech-oss/go-github-utils/pkg/commit"
	githubfileutils "github.com/form3tech-oss/go-github-utils/pkg/file"
)

const codeownersPath = ".github/CODEOWNERS"

func resourceFile() *schema.Resource {
	return &schema.Resource{
		Create: resourceFileCreate,
		Read:   resourceFileRead,
		Update: resourceFileUpdate,
		Delete: resourceFileDelete,
		Importer: &schema.ResourceImporter{
			State: resourceFileImport,
		},
		Schema: map[string]*schema.Schema{
			"repository_owner": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The repository owner e.g. my-org if the repo is my-org/my-repo",
				ForceNew:    true,
			},
			"repository_name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The repository name e.g. my-repo",
				ForceNew:    true,
			},
			"branch": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The branch to control CODEOWNERS on - defaults to the default repo branch",
				Default:     "",
				ForceNew:    true,
			},
			"rules": {
				Type:        schema.TypeList,
				ConfigMode:  schema.SchemaConfigModeAttr,
				Optional:    true,
				Description: "A list of rules that describe which reviewers should be assigned to which areas of the source code",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"pattern": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "A pattern which follows the same rules used in gitignore files",
						},
						"usernames": {
							Type:        schema.TypeSet,
							ConfigMode:  schema.SchemaConfigModeAttr,
							Required:    true,
							Description: "A list of usernames or team names using the standard @username or @org/team-name format - using the @ prefix is entirely optional",
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
							Set: schema.HashString,
						},
					},
				},
			},
		},
	}
}

func resourceFileImport(d *schema.ResourceData, m interface{}) ([]*schema.ResourceData, error) {
	err := resourceFileRead(d, m)
	return []*schema.ResourceData{d}, err
}

func resourceFileRead(d *schema.ResourceData, m interface{}) error {
	config := m.(*providerConfiguration)

	file := expandFile(d)

	ctx := context.Background()

	getOptions := &github.RepositoryContentGetOptions{
		Ref: file.Branch,
	}

	codeOwnerContent, _, rr, err := config.client.Repositories.GetContents(ctx, file.RepositoryOwner, file.RepositoryName, codeownersPath, getOptions)

	if rr != nil && rr.StatusCode == http.StatusNotFound {
		d.SetId("")
		return nil
	}

	if err != nil || rr.StatusCode >= 500 {
		return fmt.Errorf("failed to retrieve file %s: %v", codeownersPath, err)
	}

	raw, err := codeOwnerContent.GetContent()
	if err != nil {
		return fmt.Errorf("failed to retrieve content for %s: %s", codeownersPath, err)
	}

	file.Ruleset = parseRulesFile(raw)

	return flattenFile(file, d)
}

func resourceFileCreate(d *schema.ResourceData, m interface{}) error {
	return resourceFileCreateOrUpdate("Adding CODEOWNERS file", d, m)
}

func resourceFileCreateOrUpdate(s string, d *schema.ResourceData, m interface{}) error {
	config := m.(*providerConfiguration)

	file := expandFile(d)
	if file.Branch == "" {
		ctx := context.Background()
		rep, _, err := config.client.Repositories.Get(ctx, file.RepositoryOwner, file.RepositoryName)
		if err != nil {
			return err
		}
		file.Branch = *rep.DefaultBranch
	}

	entries := []*github.TreeEntry{
		{
			Path:    github.String(codeownersPath),
			Content: github.String(string(file.Ruleset.Compile())),
			Type:    github.String("blob"),
			Mode:    github.String("100644"),
		},
	}

	if err := githubcommitutils.CreateCommit(context.Background(), config.client, &githubcommitutils.CommitOptions{
		RepoOwner:                   file.RepositoryOwner,
		RepoName:                    file.RepositoryName,
		CommitMessage:               formatCommitMessage(config.commitMessagePrefix, s),
		GpgPassphrase:               config.gpgPassphrase,
		GpgPrivateKey:               config.gpgKey,
		Changes:                     entries,
		Branch:                      file.Branch,
		Username:                    config.ghUsername,
		Email:                       config.ghEmail,
		MaxRetries:                  3,
		RetryBackoff:                5 * time.Second,
		PullRequestSourceBranchName: fmt.Sprintf("terraform-provider-codeowners-%d", time.Now().UnixNano()),
		PullRequestBody:             "",
	}); err != nil {
		return err
	}

	return resourceFileRead(d, m)
}

func resourceFileUpdate(d *schema.ResourceData, m interface{}) error {
	return resourceFileCreateOrUpdate("Updating CODEOWNERS file", d, m)
}

func resourceFileDelete(d *schema.ResourceData, m interface{}) error {
	config := m.(*providerConfiguration)

	file := expandFile(d)

	// Check whether the file exists.
	_, err := githubfileutils.GetFile(context.Background(), config.client, file.RepositoryOwner, file.RepositoryName, file.Branch, codeownersPath)
	if err != nil {
		if err == githubfileutils.ErrNotFound {
			return nil
		}
		return err
	}

	// Get the tree that corresponds to the target branch.
	s, err := branch.GetSHAForBranch(context.Background(), config.client, file.RepositoryOwner, file.RepositoryName, file.Branch)
	if err != nil {
		return err
	}
	oldTree, _, err := config.client.Git.GetTree(context.Background(), file.RepositoryOwner, file.RepositoryName, s, true)
	if err != nil {
		return err
	}

	// Remove the target file from the list of entries for the new tree.
	// NOTE: Entries of type "tree" must be removed as well, otherwise deletion won't take place.
	newTree := make([]*github.TreeEntry, 0, len(oldTree.Entries))
	for _, entry := range oldTree.Entries {
		if *entry.Type != "tree" && *entry.Path != codeownersPath {
			newTree = append(newTree, entry)
		}
	}

	// Create a commit based on the new tree.
	if err := githubcommitutils.CreateCommit(context.Background(), config.client, &githubcommitutils.CommitOptions{
		RepoOwner:                   file.RepositoryOwner,
		RepoName:                    file.RepositoryName,
		CommitMessage:               formatCommitMessage(config.commitMessagePrefix, "Deleting CODEOWNERS file"),
		GpgPassphrase:               config.gpgPassphrase,
		GpgPrivateKey:               config.gpgKey,
		Changes:                     newTree,
		BaseTreeOverride:            github.String(""),
		Branch:                      file.Branch,
		Username:                    config.ghUsername,
		Email:                       config.ghEmail,
		MaxRetries:                  3,
		RetryBackoff:                5 * time.Second,
		PullRequestSourceBranchName: fmt.Sprintf("terraform-provider-codeowners-%d", time.Now().UnixNano()),
		PullRequestBody:             "",
	}); err != nil {
		return err
	}
	return nil
}

func flattenFile(file *File, d *schema.ResourceData) error {
	d.SetId(fmt.Sprintf("%s/%s:%s", file.RepositoryOwner, file.RepositoryName, file.Branch))
	if err := d.Set("repository_name", file.RepositoryName); err != nil {
		return err
	}
	if err := d.Set("repository_owner", file.RepositoryOwner); err != nil {
		return err
	}
	if err := d.Set("branch", file.Branch); err != nil {
		return err
	}
	return d.Set("rules", flattenRuleset(file.Ruleset))
}

func flattenRuleset(in Ruleset) []interface{} {
	var out []interface{}
	for _, rule := range in {
		out = append(out, map[string]interface{}{
			"pattern":   rule.Pattern,
			"usernames": schema.NewSet(schema.HashString, flattenStringList(rule.Usernames)),
		})
	}
	return out
}

func flattenStringList(list []string) []interface{} {
	vs := make([]interface{}, 0, len(list))
	sort.Strings(list)
	for _, v := range list {
		vs = append(vs, v)
	}
	return vs
}

func expandFile(d *schema.ResourceData) *File {
	file := &File{}

	file.RepositoryName = d.Get("repository_name").(string)
	file.RepositoryOwner = d.Get("repository_owner").(string)
	file.Branch = d.Get("branch").(string)

	// support imports
	if d.Id() != "" {
		parts := strings.SplitN(d.Id(), "/", 2)
		if len(parts) == 2 {
			file.RepositoryOwner = parts[0]
			subs := strings.SplitN(parts[1], ":", 2)
			if len(subs) > 0 {
				file.RepositoryName = subs[0]
				if len(subs) > 1 {
					file.Branch = subs[1]
				}
			}
		}
	}

	file.Ruleset = expandRuleset(d.Get("rules").([]interface{}))
	return file
}

func expandRuleset(in []interface{}) Ruleset {
	out := Ruleset{}
	for _, rule := range in {
		rule := rule.(map[string]interface{})
		var usernames []string
		for _, username := range rule["usernames"].(*schema.Set).List() {
			usernames = append(usernames, strings.TrimPrefix(username.(string), "@"))
		}
		sort.Strings(usernames)
		out = append(out, Rule{
			Pattern:   rule["pattern"].(string),
			Usernames: usernames,
		})
	}
	return out
}

func formatCommitMessage(p, m string) string {
	if p == "" {
		return m
	}
	return strings.TrimSpace(p) + " " + m
}
