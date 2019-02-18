package codeowners

import (
	"context"

	"github.com/google/go-github/github"
	"github.com/hashicorp/terraform/helper/schema"
	"golang.org/x/oauth2"
)

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
			"gpg_passphrase": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The passphrase associated with your gpg_secret_key, if you have provided one",
				DefaultFunc: schema.EnvDefaultFunc("GPG_PASSPHRASE", ""),
				Sensitive:   true,
			},
			"gpg_secret_key": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Description: "GPG secret key to use to sign github commits",
				DefaultFunc: schema.EnvDefaultFunc("GPG_SECRET_KEY", ""),
				Sensitive:   true,
			},
			"email": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "Email to use for commit messages - if a GPG key is provided, this email must match that used in the key",
				DefaultFunc: schema.EnvDefaultFunc("GITHUB_EMAIL", nil),
				Sensitive:   true,
			},
			"username": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "Username to use for commit messages",
				DefaultFunc: schema.EnvDefaultFunc("GITHUB_USERNAME", nil),
				Sensitive:   true,
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"codeowners_file": resourceFile(),
		},
		ConfigureFunc: providerConfigure,
	}
}

type providerConfiguration struct {
	client        *github.Client
	ghUsername    string
	ghEmail       string
	gpgKey        string
	gpgPassphrase string
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: d.Get("github_token").(string)},
	)
	tc := oauth2.NewClient(ctx, ts)

	return &providerConfiguration{
		client:        github.NewClient(tc),
		ghEmail:       d.Get("email").(string),
		ghUsername:    d.Get("username").(string),
		gpgKey:        d.Get("gpg_secret_key").(string),
		gpgPassphrase: d.Get("gpg_passphrase").(string),
	}, nil
}
