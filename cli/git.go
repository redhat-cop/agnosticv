package main

import (
	"io"
	"os/exec"
	"path/filepath"
	"bytes"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing"
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

func findMostRecentCommitCmd(p string, related []Include) *object.Commit {
	// Use the git command
	// see https://github.com/go-git/go-git/issues/137
	args := []string{
		"log",
		"--max-count=1",
		"--pretty=format:%H",
		"--",
		p,
	}

	for _, r := range related {
		args = append(args, r.path)
	}

	cmd := exec.Command("git", args...)
	logDebug.Println(cmd)

	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		logErr.Fatal(err)
	}

	repo, err := git.PlainOpenWithOptions(p, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		logErr.Fatal("Can't open repository", p, err)
	}

	commit, err := repo.CommitObject(plumbing.NewHash(out.String()))
	return commit
}
