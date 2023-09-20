package codeowners

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
)

var testAccProviders map[string]terraform.ResourceProvider
var testAccProvider *schema.Provider

func TestProvider(t *testing.T) {
	if err := Provider().InternalValidate(); err != nil {
		t.Fatalf("err: %s", err)
	}
}

func testAccPreCheck(t *testing.T) {

	required := []string{
		"GITHUB_TOKEN",
		"GITHUB_USERNAME",
		"GITHUB_EMAIL",
	}

	for _, req := range required {
		if v := os.Getenv(req); v == "" {
			t.Fatalf("%s must be set for acceptance tests", req)
		}
	}
}

func TestDefaultMergeMethod(t *testing.T) {
	if mergeMethod("wat?") != "merge" {
		t.Fatal("Unexpected default value for mergeMethod")
	}
}

func init() {
	testAccProvider = Provider()
	testAccProviders = map[string]terraform.ResourceProvider{
		"codeowners": testAccProvider,
	}
}
