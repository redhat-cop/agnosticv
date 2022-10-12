package main

import (
	"io"
	"path/filepath"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)


func isRepo(path string) bool {
	_, err := git.PlainOpenWithOptions(path, &git.PlainOpenOptions{DetectDotGit: true})
	return err == nil
}


func findMostRecentCommit(p string, related []Include) *object.Commit {
	repo, err := git.PlainOpenWithOptions(p, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		logErr.Fatal("Can't open repository", p, err)
	}

	wt, _ := repo.Worktree()

	cIter, err := repo.Log(
		&git.LogOptions{
			Order: git.LogOrderCommitterTime,
			All: false,
			PathFilter: func(path string) bool {
				if filepath.Join(wt.Filesystem.Root(), path) == abs(p) {
					return true
				}
				for _, f := range related {
					if filepath.Join(wt.Filesystem.Root(), path) == abs(f.path) {
						return true
					}
				}
				return false
			},
		},
	)

	var commit *object.Commit
	cIter.ForEach(func(o *object.Commit) error {
		commit = o
		// Stop at first found, return EOF
		return io.EOF
	})
	return commit
}
