package codeowners

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/go-github/github"
	"github.com/hashicorp/terraform/helper/schema"
)

func resourceFile() *schema.Resource {

	return &schema.Resource{
		Create: resourceFileCreate,
		Update: resourceFileUpdate,
		Read:   resourceFileRead,
		Delete: resourceFileDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema{
			"repository_owner": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "The repository owner e.g. my-org if the repo is my-org/my-repo",
				ForceNew:    true,
			},
			"repository_name": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "The repository name e.g. my-repo",
				ForceNew:    true,
			},
			"branch": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The branch to control CODEOWNERS on - defaults to the default repo branch",
				Default:     "",
			},
			"rules": &schema.Schema{
				Type:        schema.TypeSet,
				Optional:    true,
				Description: "A list of rules that describe which reviewers should be assigned to which areas of the source code",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"pattern": &schema.Schema{
							Type:        schema.TypeString,
							Required:    true,
							Description: "A pattern which follows the same rules used in gitignore files",
						},
						"usernames": &schema.Schema{
							Type:        schema.TypeSet,
							Required:    true,
							Description: "A list of usernames or team names using the standard @username or @org/team-name format - using the @ prefix is entirely optional",
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
						},
					},
				},
			},
		},
	}
}

func resourceFileRead(d *schema.ResourceData, m interface{}) error {

	config := m.(*providerConfiguration)

	file := expandFile(d)

	getOptions := &github.RepositoryContentGetOptions{}
	if d.Get("branch").(string) != "" {
		getOptions.Ref = d.Get("branch").(string)
	}

	ctx := context.Background()
	codeOwnerContent, _, rr, err := config.client.Repositories.GetContents(ctx, file.RepositoryOwner, file.RepositoryName, codeownersPath, getOptions)
	if err != nil || rr.StatusCode >= 500 {
		return fmt.Errorf("failed to retrieve file %s: %v", codeownersPath, err)
	}

	if rr.StatusCode == http.StatusNotFound {
		return fmt.Errorf("file %s does not exist", codeownersPath)
	}

	raw, err := codeOwnerContent.GetContent()
	if err != nil {
		return fmt.Errorf("failed to retrieve content for %s: %s", codeownersPath, err)
	}

	file.Ruleset = parseRulesFile(raw)

	return flattenFile(file, d)
}

func resourceFileCreate(d *schema.ResourceData, m interface{}) error {

	config := m.(*providerConfiguration)

	file := expandFile(d)

	entries := []github.TreeEntry{
		github.TreeEntry{
			Path:    github.String(codeownersPath),
			Content: github.String(string(file.Ruleset.Compile())),
			Type:    github.String("blob"),
			Mode:    github.String("100644"),
		},
	}

	if err := createCommit(config.client, &signedCommitOptions{
		repoName:      file.RepositoryOwner,
		repoOwner:     file.RepositoryName,
		branch:        file.Branch,
		commitMessage: "Adding CODEOWNERS file",
		gpgPassphrase: config.gpgPassphrase,
		gpgPrivateKey: config.gpgKey,
		username:      config.ghUsername,
		email:         config.ghEmail,
		changes:       entries,
	}); err != nil {
		return err
	}

	return resourceFileRead(d, m)
}

func resourceFileUpdate(d *schema.ResourceData, m interface{}) error {

	config := m.(*providerConfiguration)

	file := expandFile(d)

	entries := []github.TreeEntry{
		github.TreeEntry{
			Path:    github.String(codeownersPath),
			Content: github.String(string(file.Ruleset.Compile())),
			Type:    github.String("blob"),
			Mode:    github.String("100644"),
		},
	}

	if err := createCommit(config.client, &signedCommitOptions{
		repoName:      file.RepositoryOwner,
		repoOwner:     file.RepositoryName,
		branch:        file.Branch,
		commitMessage: "Updating CODEOWNERS file",
		gpgPassphrase: config.gpgPassphrase,
		gpgPrivateKey: config.gpgKey,
		username:      config.ghUsername,
		email:         config.ghEmail,
		changes:       entries,
	}); err != nil {
		return err
	}

	return resourceFileRead(d, m)
}

func resourceFileDelete(d *schema.ResourceData, m interface{}) error {
	config := m.(*providerConfiguration)

	owner, name := d.Get("repository_owner").(string), d.Get("repository_name").(string)

	ctx := context.Background()

	codeOwnerContent, _, rr, err := config.client.Repositories.GetContents(ctx, owner, name, codeownersPath, &github.RepositoryContentGetOptions{})
	if err != nil {
		return fmt.Errorf("failed to retrieve file %s: %v", codeownersPath, err)
	}

	if rr.StatusCode == http.StatusNotFound { // resource already removed
		return nil
	}

	options := &github.RepositoryContentFileOptions{
		Message: github.String("Removing CODEOWNERS file"),
		SHA:     codeOwnerContent.SHA,
	}
	if d.Get("branch").(string) != "" {
		options.Branch = github.String(d.Get("branch").(string))
	}

	_, _, err = config.client.Repositories.DeleteFile(ctx, owner, name, codeownersPath, options)
	return err
}

func flattenFile(file *File, d *schema.ResourceData) error {
	d.SetId(fmt.Sprintf("%s/%s", file.RepositoryOwner, file.RepositoryName))
	d.Set("repository_name", file.RepositoryName)
	d.Set("repository_owner", file.RepositoryOwner)
	d.Set("rules", flattenRuleset(file.Ruleset))
	d.Set("branch", file.Branch)
	return nil
}

func flattenRuleset(in Ruleset) []map[string]interface{} {
	var out = make([]map[string]interface{}, len(in), len(in))
	for _, rule := range in {
		out = append(out, map[string]interface{}{
			"pattern":   rule.Pattern,
			"usernames": rule.Usernames,
		})
	}
	return out
}

func expandFile(d *schema.ResourceData) *File {
	file := &File{}
	file.RepositoryName = d.Get("repository_name").(string)
	file.RepositoryOwner = d.Get("repository_owner").(string)
	file.Branch = d.Get("branch").(string)
	file.Ruleset = expandRuleset(d.Get("rules").(*schema.Set))
	return file
}

func expandRuleset(in *schema.Set) Ruleset {
	out := Ruleset{}
	for _, rule := range in.List() {
		rule := rule.(map[string]interface{})
		usernames := []string{}
		for _, username := range rule["usernames"].(*schema.Set).List() {
			usernames = append(usernames, username.(string))
		}
		out = append(out, Rule{
			Pattern:   rule["pattern"].(string),
			Usernames: usernames,
		})
	}
	return out
}
