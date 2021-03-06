/*
Copyright 2015 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pulls

import (
	"fmt"

	github_util "k8s.io/contrib/mungegithub/github"

	"github.com/golang/glog"
	github_api "github.com/google/go-github/github"
	"github.com/spf13/cobra"
)

// PRMunger is the interface which all mungers must implement to register
type PRMunger interface {
	// Take action on a specific pull request includes:
	//   * The config for mungers
	//   * The PR object
	//   * The issue object for the PR, github stores some things (e.g. labels) in an "issue" object with the same number as the PR
	//   * The commits for the PR
	//   * The events on the PR
	MungePullRequest(config *github_util.Config, pr *github_api.PullRequest, issue *github_api.Issue, commits []github_api.RepositoryCommit, events []github_api.IssueEvent)
	AddFlags(cmd *cobra.Command, config *github_util.Config)
	Name() string
	Initialize(*github_util.Config) error
	EachLoop(*github_util.Config) error
}

var mungerMap = map[string]PRMunger{}
var mungers = []PRMunger{}

// GetAllMungers returns a slice of all registered mungers. This list is
// completely independant of the mungers selected at runtime in --pr-mungers.
// This is all possible mungers.
func GetAllMungers() []PRMunger {
	out := []PRMunger{}
	for _, munger := range mungerMap {
		out = append(out, munger)
	}
	return out
}

// InitializeMungers will call munger.Initialize() for all mungers requested
// in --pr-mungers
func InitializeMungers(requestedMungers []string, config *github_util.Config) error {
	for _, name := range requestedMungers {
		munger, found := mungerMap[name]
		if !found {
			return fmt.Errorf("couldn't find a munger named: %s", name)
		}
		mungers = append(mungers, munger)
		if err := munger.Initialize(config); err != nil {
			return err
		}
	}
	return nil
}

// EachLoop will be called before we start a poll loop and will run the
// EachLoop function for all active mungers
func EachLoop(config *github_util.Config) error {
	for _, munger := range mungers {
		if err := munger.EachLoop(config); err != nil {
			return err
		}
	}
	return nil
}

// RegisterMunger should be called in `init()` by each munger to make itself
// available by name
func RegisterMunger(munger PRMunger) error {
	if _, found := mungerMap[munger.Name()]; found {
		return fmt.Errorf("a munger with that name (%s) already exists", munger.Name())
	}
	mungerMap[munger.Name()] = munger
	glog.Infof("Registered %#v at %s", munger, munger.Name())
	return nil
}

// RegisterMungerOrDie will call RegisterMunger but will be fatal on error
func RegisterMungerOrDie(munger PRMunger) {
	if err := RegisterMunger(munger); err != nil {
		glog.Fatalf("Failed to register munger: %s", err)
	}
}

func mungePR(config *github_util.Config, pr *github_api.PullRequest, issue *github_api.Issue) error {
	if pr == nil {
		fmt.Printf("found nil pr\n")
	}

	commits, err := config.GetFilledCommits(*pr.Number)
	if err != nil {
		return err
	}

	events, err := config.GetAllEventsForPR(*pr.Number)
	if err != nil {
		return err
	}

	for _, munger := range mungers {
		munger.MungePullRequest(config, pr, issue, commits, events)
	}
	return nil
}

// MungePullRequests is the main function which asks that each munger be called
// for each PR
func MungePullRequests(config *github_util.Config) error {
	mfunc := func(pr *github_api.PullRequest, issue *github_api.Issue) error {
		return mungePR(config, pr, issue)
	}
	if err := config.ForEachPRDo([]string{}, mfunc); err != nil {
		return err
	}

	return nil
}
