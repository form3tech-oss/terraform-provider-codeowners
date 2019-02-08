package codeowners

import (
	"context"

	"github.com/google/go-github/github"
	"github.com/hashicorp/terraform/helper/schema"
	"golang.org/x/oauth2"
)

// Provider makes the AlienVault provider available
func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"github_token": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "A github token with full repo/admin access permissions to the organisation being terraformed",
				DefaultFunc: schema.EnvDefaultFunc("GITHUB_TOKEN", nil),
				Sensitive:   true,
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"codeowners_file": resourceFile(),
		},
		ConfigureFunc: providerConfigure,
	}
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: d.Get("github_token").(string)},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	return client, nil
}
