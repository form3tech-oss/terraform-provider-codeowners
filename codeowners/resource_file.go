package codeowners

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/google/go-github/v28/github"
	"github.com/hashicorp/terraform/helper/hashcode"
	"github.com/hashicorp/terraform/helper/schema"
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
				Type:        schema.TypeSet,
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
				Set: hashRule,
			},
		},
	}
}

func hashRule(v interface{}) int {
	m := v.(map[string]interface{})
	var usernames []string
	for _, u := range m["usernames"].(*schema.Set).List() {
		usernames = append(usernames, u.(string))
	}
	sort.Strings(usernames)
	return hashcode.String(m["pattern"].(string) + strings.Join(usernames, ","))
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

	_ = flattenFile(file, d)
	return nil
}

func resourceFileCreate(d *schema.ResourceData, m interface{}) error {

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

	entries := []github.TreeEntry{
		{
			Path:    github.String(codeownersPath),
			Content: github.String(string(file.Ruleset.Compile())),
			Type:    github.String("blob"),
			Mode:    github.String("100644"),
		},
	}

	if err := createCommit(config.client, &commitOptions{
		repoOwner:     file.RepositoryOwner,
		repoName:      file.RepositoryName,
		branch:        file.Branch,
		commitMessage: formatCommitMessage(config.commitMessagePrefix, "Adding CODEOWNERS file"),
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
		{
			Path:    github.String(codeownersPath),
			Content: github.String(string(file.Ruleset.Compile())),
			Type:    github.String("blob"),
			Mode:    github.String("100644"),
		},
	}

	if err := createCommit(config.client, &commitOptions{
		repoOwner:     file.RepositoryOwner,
		repoName:      file.RepositoryName,
		branch:        file.Branch,
		commitMessage: formatCommitMessage(config.commitMessagePrefix, "Updating CODEOWNERS file"),
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

	file := expandFile(d)

	ctx := context.Background()

	codeOwnerContent, _, rr, err := config.client.Repositories.GetContents(ctx, file.RepositoryOwner, file.RepositoryName, codeownersPath, &github.RepositoryContentGetOptions{Ref: file.Branch})
	if err != nil {
		return fmt.Errorf("failed to retrieve file %s: %v", codeownersPath, err)
	}

	if rr.StatusCode == http.StatusNotFound { // resource already removed
		return nil
	}

	options := &github.RepositoryContentFileOptions{
		Message: github.String(formatCommitMessage(config.commitMessagePrefix, "Removing CODEOWNERS file")),
		SHA:     codeOwnerContent.SHA,
	}
	if file.Branch != "" {
		options.Branch = &file.Branch
	}

	_, _, err = config.client.Repositories.DeleteFile(ctx, file.RepositoryOwner, file.RepositoryName, codeownersPath, options)
	return err
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

	file.Ruleset = expandRuleset(d.Get("rules").(*schema.Set))
	return file
}

func expandRuleset(in *schema.Set) Ruleset {
	out := Ruleset{}
	for _, rule := range in.List() {
		rule := rule.(map[string]interface{})
		var usernames []string
		for _, username := range rule["usernames"].(*schema.Set).List() {
			usernames = append(usernames, username.(string))
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
