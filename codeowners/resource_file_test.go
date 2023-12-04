package codeowners

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-github/v54/github"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
)

const testAccFileConfig = `
	resource "codeowners_file" "my-codeowners-file" {
		repository_name  = "enforcement-test-repo"
		repository_owner = "form3tech-oss"
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

const testAccFileConfigUpdate = `
	resource "codeowners_file" "my-codeowners-file" {
		repository_name  = "enforcement-test-repo"
		repository_owner = "form3tech-oss"
		rules = [
			{
				pattern = "*"
				usernames = [ "expert" ]
			},
			{
				pattern = "*.go"
				usernames = [ "go-expert" ]
			},
			{
				pattern = "*.java"
				usernames = [ "java-expert", "java-guru", "someone-else" ]
			}
		]
	}`

func TestAccResourceFile_basic(t *testing.T) {
	var before, after File

	resourceName := "codeowners_file.my-codeowners-file"

	resource.Test(t, resource.TestCase{
		PreCheck:      func() { testAccPreCheck(t) },
		IDRefreshName: resourceName,
		Providers:     testAccProviders,
		CheckDestroy:  testAccCheckFileDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccFileConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckFileExists(resourceName, &before),
					resource.TestCheckResourceAttr(resourceName, "rules.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "rules.0.pattern", "*"),
					resource.TestCheckResourceAttr(resourceName, "rules.0.usernames.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rules.0.usernames.1327207234", "expert"),
					resource.TestCheckResourceAttr(resourceName, "rules.1.pattern", "*.java"),
					resource.TestCheckResourceAttr(resourceName, "rules.1.usernames.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "rules.1.usernames.2414450220", "java-guru"),
					resource.TestCheckResourceAttr(resourceName, "rules.1.usernames.680681689", "java-expert"),
					resource.TestCheckResourceAttr(resourceName, "repository_name", "enforcement-test-repo"),
					resource.TestCheckResourceAttr(resourceName, "repository_owner", "form3tech-oss"),
					resource.TestCheckResourceAttr(resourceName, "branch", ""),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccFileConfigUpdate,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckFileExists(resourceName, &after),
					resource.TestCheckResourceAttr(resourceName, "rules.#", "3"),
					resource.TestCheckResourceAttr(resourceName, "rules.0.pattern", "*"),
					resource.TestCheckResourceAttr(resourceName, "rules.0.usernames.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rules.0.usernames.1327207234", "expert"),
					resource.TestCheckResourceAttr(resourceName, "rules.1.pattern", "*.go"),
					resource.TestCheckResourceAttr(resourceName, "rules.1.usernames.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rules.1.usernames.2272469097", "go-expert"),
					resource.TestCheckResourceAttr(resourceName, "rules.2.pattern", "*.java"),
					resource.TestCheckResourceAttr(resourceName, "rules.2.usernames.#", "3"),
					resource.TestCheckResourceAttr(resourceName, "rules.2.usernames.2414450220", "java-guru"),
					resource.TestCheckResourceAttr(resourceName, "rules.2.usernames.680681689", "java-expert"),
					resource.TestCheckResourceAttr(resourceName, "rules.2.usernames.504743642", "someone-else"),
					resource.TestCheckResourceAttr(resourceName, "repository_name", "enforcement-test-repo"),
					resource.TestCheckResourceAttr(resourceName, "repository_owner", "form3tech-oss"),
					resource.TestCheckResourceAttr(resourceName, "branch", ""),
				),
			},
		},
	})
}

const testAccFileConfigLeadingAtSign = `
	resource "codeowners_file" "my-codeowners-file" {
		repository_name  = "enforcement-test-repo"
		repository_owner = "form3tech-oss"
		rules = [
			{
				pattern = "*"
				usernames = [ "@expert" ]
			},
			{
				pattern = "*.java"
				usernames = [ "java-expert", "@java-guru" ]
			}
		]
	}`

func TestAccResourceFile_OptionalAtSign(t *testing.T) {
	resourceName := "codeowners_file.my-codeowners-file"

	resource.Test(t, resource.TestCase{
		PreCheck:      func() { testAccPreCheck(t) },
		IDRefreshName: resourceName,
		Providers:     testAccProviders,
		CheckDestroy:  testAccCheckFileDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccFileConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "rules.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "rules.0.pattern", "*"),
					resource.TestCheckResourceAttr(resourceName, "rules.0.usernames.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rules.0.usernames.1327207234", "expert"),
					resource.TestCheckResourceAttr(resourceName, "rules.1.pattern", "*.java"),
					resource.TestCheckResourceAttr(resourceName, "rules.1.usernames.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "rules.1.usernames.2414450220", "java-guru"),
					resource.TestCheckResourceAttr(resourceName, "rules.1.usernames.680681689", "java-expert"),
					resource.TestCheckResourceAttr(resourceName, "repository_name", "enforcement-test-repo"),
					resource.TestCheckResourceAttr(resourceName, "repository_owner", "form3tech-oss"),
					resource.TestCheckResourceAttr(resourceName, "branch", ""),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config:   testAccFileConfigLeadingAtSign,
				PlanOnly: true,
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
		owner, nameBranch := parts[0], parts[1]
		sub := strings.Split(nameBranch, ":")
		name := sub[0]
		branch := sub[1]

		ctx := context.Background()
		_, _, response, err := config.client.Repositories.GetContents(ctx, owner, name, codeownersPath, &github.RepositoryContentGetOptions{Ref: branch})
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

		config := testAccProvider.Meta().(*providerConfiguration)

		parts := strings.Split(rs.Primary.ID, "/")
		if len(parts) != 2 {
			return fmt.Errorf("Invalid ID")
		}
		owner, nameBranch := parts[0], parts[1]
		sub := strings.Split(nameBranch, ":")
		name := sub[0]
		branch := sub[1]

		ctx := context.Background()
		codeOwnerContent, _, rr, err := config.client.Repositories.GetContents(ctx, owner, name, codeownersPath, &github.RepositoryContentGetOptions{Ref: branch})
		if err != nil || rr.StatusCode >= 500 {
			return fmt.Errorf("failed to retrieve file %s: %v", codeownersPath, err)
		}

		if rr.StatusCode == http.StatusNotFound {
			return fmt.Errorf("file %s does not exist", codeownersPath)
		}

		file := &File{
			RepositoryOwner: owner,
			RepositoryName:  name,
			Branch:          branch,
		}

		raw, err := codeOwnerContent.GetContent()
		if err != nil {
			return err
		}
		file.Ruleset = parseRulesFile(raw)

		*res = *file
		return nil
	}
}
