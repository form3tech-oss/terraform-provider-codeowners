package codeowners

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/liamg/go-github/github"
	"golang.org/x/crypto/openpgp"
)

type commitOptions struct {
	repoOwner     string
	repoName      string
	commitMessage string
	gpgPassphrase string
	gpgPrivateKey string // detached armor format
	changes       []github.TreeEntry
	branch        string
	username      string
	email         string
}

func createCommit(client *github.Client, options *commitOptions) error {

	ctx := context.Background()
	branch := options.branch

	if branch == "" {
		rep, _, err := client.Repositories.Get(ctx, options.repoOwner, options.repoName)
		if err != nil {
			return err
		}
		branch = *rep.DefaultBranch
	}

	// get ref for selected branch
	ref, _, err := client.Git.GetRef(ctx, options.repoOwner, options.repoName, "refs/heads/"+branch)
	if err != nil {
		return err
	}

	// create tree containing required changes
	tree, _, err := client.Git.CreateTree(ctx, options.repoOwner, options.repoName, *ref.Object.SHA, options.changes)
	if err != nil {
		return err
	}

	// get parent commit
	parent, _, err := client.Repositories.GetCommit(ctx, options.repoOwner, options.repoName, *ref.Object.SHA)
	if err != nil {
		return err
	}

	// This is not always populated, but is needed.
	parent.Commit.SHA = github.String(parent.GetSHA())

	date := time.Now()
	author := &github.CommitAuthor{
		Date:  &date,
		Name:  github.String(options.username),
		Email: github.String(options.email),
	}

	commit := &github.Commit{
		Author:  author,
		Message: &options.commitMessage,
		Tree:    tree,
		Parents: []github.Commit{*parent.Commit},
	}

	if options.gpgPrivateKey != "" {
		if err := signCommit(commit, options.gpgPrivateKey, options.gpgPassphrase); err != nil {
			return err
		}
	}

	newCommit, _, err := client.Git.CreateCommit(ctx, options.repoOwner, options.repoName, commit)
	if err != nil {
		return err
	}

	// Attach the commit to the selected branch
	ref.Object.SHA = newCommit.SHA
	_, _, err = client.Git.UpdateRef(ctx, options.repoOwner, options.repoName, ref, false)
	return err
}

func signCommit(commit *github.Commit, privateKey string, passphrase string) error {

	// the payload must be "an over the string commit as it would be written to the object database"
	// we sign this data to verify the commit
	payload := fmt.Sprintf(
		`tree %s
parent %s
author %s <%s> %d +0000
committer %s <%s> %d +0000

%s`,
		commit.Tree.GetSHA(),
		commit.Parents[0].GetSHA(),
		commit.Author.GetName(),
		commit.Author.GetEmail(),
		commit.Author.Date.Unix(),
		commit.Author.GetName(),
		commit.Author.GetEmail(),
		commit.Author.Date.Unix(),
		commit.GetMessage(),
	)

	// sign the payload data
	signature, err := createSignature(payload, privateKey, passphrase)
	if err != nil {
		return err
	}

	commit.Verification = &github.SignatureVerification{
		Signature: signature,
	}

	return nil
}

func createSignature(data string, privateKey string, passphrase string) (*string, error) {

	entitylist, err := openpgp.ReadArmoredKeyRing(strings.NewReader(privateKey))
	if err != nil {
		return nil, err
	}
	pk := entitylist[0]

	ppb := []byte(passphrase)

	if pk.PrivateKey != nil && pk.PrivateKey.Encrypted {
		err := pk.PrivateKey.Decrypt(ppb)
		if err != nil {
			return nil, err
		}
	}

	for _, subkey := range pk.Subkeys {
		if subkey.PrivateKey != nil && subkey.PrivateKey.Encrypted {
			err := subkey.PrivateKey.Decrypt(ppb)
			if err != nil {
				return nil, err
			}
		}
	}

	out := new(bytes.Buffer)
	reader := strings.NewReader(data)
	if err := openpgp.ArmoredDetachSign(out, pk, reader, nil); err != nil {
		return nil, err
	}
	signature := string(out.Bytes())
	return &signature, nil
}
