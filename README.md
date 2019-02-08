# AlienVault Terraform Provider

[![Build Status](https://travis-ci.org/form3tech-oss/terraform-provider-codeowners.svg?branch=master)](https://travis-ci.org/form3tech-oss/terraform-provider-codeowners)

Terraform Provider for GitHub [CODEOWNERS](https://help.github.com/articles/about-code-owners/) files.

## Summary

Do you use terraform to manage your GitHub organisation? Are you frustrated that you can't manage your code review approvers using the same method? Well, now you can!

## Installation

Download the relevant binary from [releases](https://github.com/form3tech-oss/terraform-provider-codeowners/releases) and copy it to `$HOME/.terraform.d/plugins/`.

## Authentication

There are two methods for authenticating with this provider.

You can specify your github token in the `provider` block, as below:

```hcl
provider "codeowners" {
    github_token = "..."
}
```

Alternatively, you can use the following environment variable:

```bash
export GITHUB_TOKEN="..."
```

Provider block variables will override environment variables, where provided.

## Resources

### `codeowners_file`

```hcl
```