package reporter

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/pkg/errors"
	"github.com/sethvargo/go-githubactions"
)

type Opt interface {
	apply(r *Reporter) error
}

type Option func(r *Reporter)

func (o Option) apply(r *Reporter) error { o(r); return nil }

type OptionWithError func(r *Reporter) error

func (o OptionWithError) apply(r *Reporter) error { return o(r) }

// ---------------------------------------
// Coverage files section.
// ---------------------------------------

// WithCoverFilepath sets the coverage file path for the Reporter instance. Only
// one of WithCoverFilepath, WithCoverageFileReader, WithCoverageFileString, or
// WithCoverageFileBytes should be set.
func WithCoverFilepath(path string) OptionWithError {
	return func(r *Reporter) (err error) {
		path = filepath.Join(r.root, path)
		file, err := os.Open(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return errors.Errorf("file %q does not exist", path)
			}

			return errors.WithStack(err)
		}

		r.coverageFile = file

		return nil
	}
}

// WithCoverageFileReader sets the coverage file contents for the Reporter
// instance. Only one of WithCoverFilepath, WithCoverageFileReader,
// WithCoverageFileString, or WithCoverageFileBytes should
func WithCoverageFileReader(file io.Reader) Option {
	return func(r *Reporter) { r.coverageFile = file }
}

// WithCoverageFileString sets the coverage file contents for the Reporter
// instance. Only one of WithCoverFilepath, WithCoverageFileReader,
// WithCoverageFileString, or WithCoverageFileBytes should
func WithCoverageFileString(file string) Option {
	return func(r *Reporter) { r.coverageFile = strings.NewReader(file) }
}

// WithCoverageFileBytes sets the coverage file contents for the Reporter
// instance. Only one of WithCoverFilepath, WithCoverageFileReader,
// WithCoverageFileString, or WithCoverageFileBytes should
func WithCoverageFileBytes(file []byte) Option {
	return func(r *Reporter) { r.coverageFile = bytes.NewReader(file) }
}

// WithOldCoverage sets the previous coverage percentage to be used in the
// Reporter. Required for calculating total coverage difference. Ignored if
// WithOldCoverFilepath is set.
func WithOldCoverage(cov float64) Option { return func(r *Reporter) { r.oldCoverage = cov } }

// WithOldCoverFilepath sets the path to the old coverage file in the Reporter
// configuration. Required for calculating total coverage difference. Ignores
// the WithOldCoverage option if set.
func WithOldCoverFilepath(path string) Option { return func(r *Reporter) { r.oldCoverFile = path } }

// ---------------------------------------
// Ignored files from coverage section.
// ---------------------------------------

// WithSkipMain configures the Reporter to skip coverage for files in the main
// package based on the provided boolean value. Defaults to false.
func WithSkipMain(skip bool) Option { return func(r *Reporter) { r.skipMain = skip } }

// WithSkipGenerated sets whether files with generated code should be skipped of
// coverage calculations. Defaults to false.
func WithSkipGenerated(skip bool) Option { return func(r *Reporter) { r.skipGenerated = skip } }

// WithIgnoreFilepath sets the file path for coverage ignore rules in the
// Reporter configuration. The file should contain one ignore rule per line. You
// can use "#" to comment out lines.
func WithIgnoreFilepath(path string) OptionWithError {
	return func(r *Reporter) error {
		path = filepath.Join(r.root, path)
		ignore, err := ParseIgnoreFile(path)
		if err != nil {
			return errors.Wrapf(err, "parsing %q", path)
		}

		r.ignore = ignore
		return nil
	}
}

// ---------------------------------------
// Comment posting configuration section.
// ---------------------------------------

// WithGithubToken sets the GitHub token for the Reporter to enable API calls
// requiring authentication. Required for posting comments to pull requests.
func WithGithubToken(token string) Option { return func(r *Reporter) { r.githubToken = token } }

// WithPRNumber sets the pull request number in the Reporter configuration.
// Required for posting comments to pull requests. The Reporter will attempt to
// retrieve the pull request from the GitHub API, but this may fail in some
// cases.
func WithPRNumber(pr int) Option { return func(r *Reporter) { r.prNumber = pr } }

// WithRepoURL sets the repository URL for the Reporter instance. Required for
// posting comments to pull requests. This is used to construct the link to the
// file in the pull request, e.g.,
// https://github.com/owner/repo/blob/branch/file.go#L123-L125.
func WithRepoURL(url string) Option { return func(r *Reporter) { r.repoURL = url } }

// ---------------------------------------
// Comment body configuration section.
// ---------------------------------------

// WithTemplatePath is an Option that sets the path to the template file for the
// Reporter. If not set, the DefaultTemplate will be used.
func WithTemplatePath(path string) Option { return func(r *Reporter) { r.templatePath = path } }

// WithHideTotal sets the hideTotal flag in the Reporter to indicate whether the
// total coverage section should be hidden in the generated comment. Defaults to
// false.
func WithHideTotal(hide bool) Option { return func(r *Reporter) { r.hideTotal = hide } }

// WithHidePatch configures the Reporter to hide or show the patch coverage
// section in the generated comment. Defaults to false.
func WithHidePatch(hide bool) Option { return func(r *Reporter) { r.hidePatch = hide } }

// WithHideHowToImprove sets whether the "How to improve" section should be
// hidden in the coverage report comment. Defaults to false.
func WithHideHowToImprove(hide bool) Option { return func(r *Reporter) { r.hideHowToImprove = hide } }

// WithHowToImproveLimit sets a limit on the number of uncovered changes
// displayed in the coverage report comments. It is used to prevent the report
// from exceeding the GitHub comment character limit. The limit is applied after
// sorting the changes by coverage difference, the most uncovered changes will
// be displayed first. Defaults to 100. Set to -1 to disable the limit.
func WithHowToImproveLimit(limit int) Option {
	return func(r *Reporter) { r.howToImproveLimit = limit }
}

// ---------------------------------------
// Test related options.
// ---------------------------------------

// WithRoot specifies the root directory of the module for the Reporter.
// Defaults to the current working directory.
func WithRoot(root string) Option { return func(r *Reporter) { r.root = root } }

// WithRepo sets the git.Repository instance for the Reporter. If not set, the
// Reporter will attempt to retrieve the repository from the current working
// directory.
func WithRepo(repo *git.Repository) Option { return func(r *Reporter) { r.repo = repo } }

// WithGithubAction sets a GitHub Actions client for the Reporter to enable
// interaction with GitHub workflows. If not set, the Reporter will attempt to
// retrieve the client from the environment.
func WithGithubAction(a *githubactions.Action) Option { return func(r *Reporter) { r.action = a } }
