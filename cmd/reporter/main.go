package main

import (
	"context"
	_ "embed"
	"flag"

	"github.com/psyhatter/go-coverage-report/reporter"
	"github.com/sethvargo/go-githubactions"
)

var (
	branchName       = flag.String("branch", "main", "branch name")
	changesLimit     = flag.Int("changes-limit", 100, "limit number of changes to show in comment")
	coverFile        = flag.String("coverage-file", "coverage.out", "coverage file")
	githubToken      = flag.String("github-token", "", "github token to post comment, skip posting if not set")
	hideHowToImprove = flag.Bool("hide-how-to-improve", false, "hide how to improve section in comment")
	hidePatch        = flag.Bool("hide-patch", false, "hide patch coverage section in comment")
	hideTotal        = flag.Bool("hide-total", false, "hide total coverage section in comment")
	oldCoverage      = flag.Float64("prev-coverage", 0, "previous coverage value")
	oldCoverFile     = flag.String("prev-coverage-file", "", "previous coverage file")
	prNumber         = flag.Int("pr-number", 0, "pull request number")
	repoURL          = flag.String("repo-url", "", "repository url")
	root             = flag.String("root", ".", "root of the module")
	skipGenerated    = flag.Bool("skip-generated", false, "skip generated files")
	skipMain         = flag.Bool("skip-package-main", false, "skip main package files")
	templatePath     = flag.String("comment-template-file", "", "path to template file")
	coverIgnoreFile  = flag.String("cover-ignore-filepath", ".coverignore", "path to cover ignore file")
)

func main() {
	flag.Parse()

	action := githubactions.New()

	r, err := reporter.New(
		*branchName,
		reporter.WithRoot(*root),
		reporter.WithRepoURL(*repoURL),
		reporter.WithPRNumber(*prNumber),
		reporter.WithSkipMain(*skipMain),
		reporter.WithGithubAction(action),
		reporter.WithHideTotal(*hideTotal),
		reporter.WithHidePatch(*hidePatch),
		reporter.WithOldCoverage(*oldCoverage),
		reporter.WithGithubToken(*githubToken),
		reporter.WithCoverFilepath(*coverFile),
		reporter.WithTemplatePath(*templatePath),
		reporter.WithSkipGenerated(*skipGenerated),
		reporter.WithOldCoverFilepath(*oldCoverFile),
		reporter.WithIgnoreFilepath(*coverIgnoreFile),
		reporter.WithHowToImproveLimit(*changesLimit),
		reporter.WithHideHowToImprove(*hideHowToImprove),
	)
	if err != nil {
		action.Fatalf("%+v", err)
	}

	err = r.Execute(context.Background())
	if err != nil {
		action.Fatalf("%+v", err)
	}
}
