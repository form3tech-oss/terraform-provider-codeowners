package main

import (
	"github.com/form3tech-oss/terraform-provider-codeowners/codeowners"
	"github.com/hashicorp/terraform-plugin-sdk/plugin"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: func() terraform.ResourceProvider {
			return codeowners.Provider()
		},
	})
}
