package main

import (
	"bytes"
	"fmt"
	"github.com/VictorLowther/go-git/git"
	"regexp"
	"strings"
)

var (
	repo *git.Repo
)

type author struct {
	Name  string
	Email string
}

func (a *author) String() string {
	return fmt.Sprintf("%s <%s>", a.Name, a.Email)
}

type pageRevision struct {
	Title       string
	Object      string
	Description string
	Author      author
	Timestamp   string
}

type searchResult struct {
	Title   string
	Content string
}

func title(file string) string {
	return strings.Replace(file, ".md", "", -1)
}

func gitExec(command string, args ...string) (*bytes.Buffer, error) {
	res, out, stderr := repo.Git(command, args...)
	runErr := res.Run()
	if runErr != nil {
		return out, runErr
	} else if stderr.Len() > 0 {
		return out, fmt.Errorf(stderr.String())
	}
	return out, nil
}

func gitShow(file string, revision string) (*bytes.Buffer, error) {
	return gitExec("show", fmt.Sprintf("%s:%s", revision, file))
}

func gitAdd(file string) (*bytes.Buffer, error) {
	return gitExec("add", file)
}

func gitCommit(message string, author author) (*bytes.Buffer, error) {
	if author.String() == "" {
		return gitExec("commit", "-m", message)
	}
	return gitExec("commit", "-m", message, "--author", author.String())
}

func gitLog(file string) ([]pageRevision, error) {
	var revisions []pageRevision
	out, err := gitExec("log", "--pretty=format:%h %an <%ae> %ad %s", "--date=relative", file)
	if err != nil {
		return revisions, err
	}
	var data []byte
	for err == nil {
		data, err = out.ReadBytes('\n')
		revision := parseGitLog(string(data))
		if revision.Object == "" {
			continue
		}
		revision.Title = title(file)
		revisions = append(revisions, revision)
	}
	return revisions, nil
}

func parseGitLog(log string) pageRevision {
	re := regexp.MustCompile(`(.{0,7}) (.+) (<.+>) (\d+ \w+ ago) (.*)`)
	matches := re.FindStringSubmatch(log)
	if len(matches) == 6 {
		return pageRevision{Object: matches[1], Author: author{Name: matches[2], Email: matches[3]}, Timestamp: matches[4], Description: matches[5]}
	}
	return pageRevision{}
}

func gitGrep(keyword string) ([]searchResult, error) {
	var results []searchResult
	out, err := gitExec("grep", "--ignore-case", keyword)
	if err != nil {
		return results, err
	}
	results = parseGitGrep(out)
	return results, nil
}

func parseGitGrep(output *bytes.Buffer) []searchResult {
	var err error
	var bytes []byte
	results := make([]searchResult, 0)

	re := regexp.MustCompile(`(.+)\.md:(.*)`)
	for err == nil {
		bytes, err = output.ReadBytes('\n')
		matches := re.FindStringSubmatch(string(bytes))
		if len(matches) == 3 {
			results = append(results, searchResult{Title: matches[1], Content: matches[2]})
		}
	}
	return results
}
