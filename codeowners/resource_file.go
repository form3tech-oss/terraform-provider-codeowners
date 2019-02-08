package codeowners

import (
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

	client := m.(*github.Client)

	file := &File{
		RepositoryOwner: d.Get("repository_owner").(string),
		RepositoryName:  d.Get("repository_name").(string),
	}

	ruleset, err := readRulesetForRepo(client, file.RepositoryOwner, file.RepositoryName)
	if err != nil {
		return err
	}

	file.Ruleset = ruleset

	return flattenFile(file, d)
}

func resourceFileCreate(d *schema.ResourceData, m interface{}) error {

	client := m.(*github.Client)

	file := expandFile(d)

	if err := createRulesetForRepo(client, file.RepositoryOwner, file.RepositoryName, file.Ruleset, "Adding CODEOWNERS file"); err != nil {
		return err
	}

	return resourceFileRead(d, m)
}

func resourceFileUpdate(d *schema.ResourceData, m interface{}) error {

	client := m.(*github.Client)

	file := expandFile(d)

	if err := updateRulesetForRepo(client, file.RepositoryOwner, file.RepositoryName, file.Ruleset, "Adding CODEOWNERS file"); err != nil {
		return err
	}

	return resourceFileRead(d, m)
}

func resourceFileDelete(d *schema.ResourceData, m interface{}) error {
	client := m.(*github.Client)

	owner, name := d.Get("repository_owner").(string), d.Get("repository_name").(string)

	return deleteRulesetForRepo(client, owner, name, "Removing CODEOWNERS file")
}

func flattenFile(file *File, d *schema.ResourceData) error {
	d.SetId(file.RepositorySlug())
	d.Set("repository_name", file.RepositoryName)
	d.Set("repository_owner", file.RepositoryOwner)
	d.Set("rules", flattenRuleset(file.Ruleset))
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
