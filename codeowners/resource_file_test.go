package codeowners

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-github/github"
	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/terraform"
)

const testAccFileConfig = `
	resource "codeowners_file" "my-codeowners-file" {
		repository_name  = "enforcement-test-repo"
		repository_owner = "form3tech"
		branch           = "master"
		rules = [
			{
				pattern = "*"
				usernames = [ "expert" ]
			},
			{
				pattern = "*.java"
				usernames = [ "java-expert", "java-guru" ]
			}
		]
	}`

func TestAccResourceFile(t *testing.T) {
	var file File

	expectedRuleset := Ruleset{
		{Pattern: "*", Usernames: []string{"expert"}},
		{Pattern: "*.java", Usernames: []string{"java-expert", "java-guru"}},
	}

	resource.Test(t, resource.TestCase{
		PreCheck:      func() { testAccPreCheck(t) },
		IDRefreshName: "codeowners_file.my-codeowners-file",
		Providers:     testAccProviders,
		CheckDestroy:  testAccCheckFileDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccFileConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckFileExists("codeowners_file.my-codeowners-file", &file),
					testAccCheckRules(&file, expectedRuleset),
					resource.TestCheckResourceAttr("codeowners_file.my-codeowners-file", "repository_name", "enforcement-test-repo"),
					resource.TestCheckResourceAttr("codeowners_file.my-codeowners-file", "repository_owner", "form3tech"),
				),
			},
		},
	})
}

func testAccCheckFileDestroy(s *terraform.State) error {
	config := testAccProvider.Meta().(*providerConfiguration)

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "codeowners_repository_file" {
			continue
		}

		parts := strings.Split(rs.Primary.ID, "/")
		if len(parts) != 2 {
			return fmt.Errorf("Invalid ID")
		}
		owner, name := parts[0], parts[1]

		ctx := context.Background()
		_, _, response, err := config.client.Repositories.GetContents(ctx, owner, name, codeownersPath, &github.RepositoryContentGetOptions{})
		if err != nil || response.StatusCode >= 500 {
			return err
		}

		if response.StatusCode != http.StatusNotFound {
			return fmt.Errorf("codeowners file for %q still exists", rs.Primary.ID)
		}
	}

	return nil
}

func testAccCheckFileExists(n string, res *File) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return errors.New("no resource ID is set")
		}

		client := testAccProvider.Meta().(*github.Client)

		parts := strings.Split(rs.Primary.ID, "/")
		if len(parts) != 2 {
			return fmt.Errorf("Invalid ID")
		}
		owner, name := parts[0], parts[1]

		ctx := context.Background()
		codeOwnerContent, _, rr, err := client.Repositories.GetContents(ctx, owner, name, codeownersPath, &github.RepositoryContentGetOptions{})
		if err != nil || rr.StatusCode >= 500 {
			return fmt.Errorf("failed to retrieve file %s: %v", codeownersPath, err)
		}

		if rr.StatusCode == http.StatusNotFound {
			return fmt.Errorf("file %s does not exist", codeownersPath)
		}

		file := &File{
			RepositoryOwner: owner,
			RepositoryName:  name,
		}

		raw, err := codeOwnerContent.GetContent()
		if err != nil {
			return err
		}
		file.Ruleset = parseRulesFile(raw)

		_, err = codeOwnerContent.GetContent()
		if err != nil {
			return fmt.Errorf("failed to retrieve content for %s: %s", codeownersPath, err)
		}

		*res = *file
		return nil
	}
}

func testAccCheckRules(res *File, expectedRuleset Ruleset) resource.TestCheckFunc {
	return func(s *terraform.State) error {

		if !expectedRuleset.Equal(res.Ruleset) {
			return fmt.Errorf("Rulesets were not equal: \n  expected=%#v\n  actual=%#v", expectedRuleset, res.Ruleset)
		}

		return nil
	}
}
