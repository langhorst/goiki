package main

import (
	"github.com/VictorLowther/go-git/git"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"testing"
)

func initRepo() string {
	dir, _ := ioutil.TempDir("", "")
	cmd := exec.Command("git", "init", dir)
	err := cmd.Run()
	if err != nil {
		panic(err)
	}
	repo, err = git.Open(dir)
	if err != nil {
		panic(err)
	}
	return dir
}

func discardRepo(dir string) {
	cmd := exec.Command("rm", "-rf", dir)
	err := cmd.Run()
	if err != nil {
		panic(err)
	}
}

func TestGit(t *testing.T) {
	dir := initRepo()
	defer discardRepo(dir)

	file := "test.txt"
	data := "Testing adding and committing."
	ioutil.WriteFile(filepath.Join(dir, file), []byte(data), 0600)
	out, err := gitAdd(file)
	if out.Len() > 0 || err != nil {
		t.Errorf(`Unable to add file %s to repository at %s, output: "%v", error: "%v"`, file, dir, out, err)
	}

	message := "Test commit"
	author := author{Name: "Test", Email: "test@example.com"}
	out, err = gitCommit(message, author)
	if err != nil {
		t.Errorf(`Unable to commit message "%s" from author %s`, message, author.String())
	}

	head, _ := gitShow(file, "HEAD")
	if head.String() != data {
		t.Errorf(`Content from file %s in HEAD revision should equal "%s" but was "%s"`, file, data, head.String())
	}

	revisions, _ := gitLog(file)
	if len(revisions) != 1 {
		t.Errorf(`Number of revisions returned should equal 1, but was %d`, len(revisions))
	}

	rev, _ := gitShow(file, revisions[0].Object)
	if rev.String() != data {
		t.Errorf(`Content from file %s in %s revision should equal "%s" but was "%s"`, file, revisions[0].Object, data, rev.String())
	}

	keyword := "adding"
	results, _ := gitGrep(keyword)
	if len(results) != 1 {
		t.Errorf(`Number of results returned should equal 1, but was %d`, len(results))
	}
}
