package main

import (
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)


func isRepo(path string) bool {
	_, err := git.PlainOpenWithOptions(path, &git.PlainOpenOptions{DetectDotGit: true})
	return err == nil
}

func GetCommit(p string) (*object.Commit) {

	repo, err := git.PlainOpenWithOptions(p, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		logErr.Fatal("Can't open repository", p, err)
	}

	ref, err := repo.Head()
	if err != nil {
		logErr.Fatal("Can't read HEAD", err)
	}

	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		logErr.Fatal("Can't read log of", ref.Hash())
	}

	return commit
}
