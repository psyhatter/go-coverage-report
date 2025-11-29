package reporter

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/pkg/errors"
	"github.com/sethvargo/go-githubactions"
)

func New(branchName string, opts ...Opt) (r *Reporter, err error) {
	r = &Reporter{
		root:              ".",
		howToImproveLimit: 100,
		branchName:        branchName,
		action:            githubactions.New(),
	}

	for _, opt := range opts {
		if err = opt.apply(r); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	if r.repo == nil {
		r.repo, err = git.PlainOpen(r.root)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}

	return r, nil
}

type Reporter struct {
	action *githubactions.Action

	coverageFile io.Reader
	repo         *git.Repository
	ignore       []*regexp.Regexp

	howToImproveLimit int
	prNumber          int

	oldCoverage float64

	root         string
	branchName   string
	oldCoverFile string
	githubToken  string
	repoURL      string
	templatePath string

	skipGenerated    bool
	skipMain         bool
	hideTotal        bool
	hidePatch        bool
	hideHowToImprove bool
}

func (r Reporter) Execute(ctx context.Context) error {
	c, err := r.action.Context()
	if err != nil {
		return errors.WithStack(err)
	}

	if r.coverageFile == nil {
		return errors.New("coverage file must be set via options")
	}

	r.action.Group("Parsed ignorable expressions")
	for _, re := range r.ignore {
		r.action.Infof("%s", re)
	}
	r.action.EndGroup()

	module, err := GetModuleName(r.root)
	if err != nil {
		return err
	}

	headCommit, err := GetHeadCommit(r.repo)
	if err != nil {
		return err
	}

	branchCommit, err := GetBranchCommit(r.repo, r.branchName)
	if err != nil {
		return err
	}

	changes, err := GetChanges(ctx, r.action, module, branchCommit, headCommit, r.ignore, r.skipGenerated, r.skipMain)
	if err != nil {
		return err
	}

	statements, err := ParseCoverageFile(r.coverageFile, module, headCommit, r.ignore, r.skipGenerated, r.skipMain)
	if err != nil {
		return err
	}

	changes = Merge(changes, statements)

	total := TotalCoverage(statements)
	patch := PatchCoverage(changes)

	old, err := OldCoverage(r.action, r.root, r.oldCoverFile, branchCommit, r.ignore, r.skipGenerated, r.skipMain)
	if err != nil {
		return err
	}

	// In case there are too many changes without covering, we limit their number for comment.
	changes = UncoveredChanges(changes, r.howToImproveLimit)

	if old == 0 {
		old = r.oldCoverage
	}

	if c != nil && c.RunID != 0 {
		r.action.SetOutput("total", fmt.Sprintf("%.1f", total))
		r.action.SetOutput("patch", fmt.Sprintf("%.1f", patch))
		r.action.SetOutput("prev-total", fmt.Sprintf("%.1f", old))
		r.action.SetOutput("diff", fmt.Sprintf("%.1f", total-old))
	}

	if old == 0 {
		r.action.Infof("Total: %.1f%%", total)
	} else {
		r.action.Infof("Old: %.1f%%", old)
		r.action.Infof("New: %.1f%%", total)
		r.action.Infof("Diff: %.1f%%", total-old)
	}
	r.action.Infof("Changes covered: %.1f%%", patch)
	r.action.Group("Uncovered changes")
	for filename, blocks := range changes {
		for _, block := range blocks {
			r.action.Infof("%s:%d-%d", filename.Value(), block.Start, block.End)
		}
	}
	r.action.EndGroup()

	params := GitHubParams{Token: r.githubToken, PRNumber: r.prNumber}
	if ss := strings.SplitN(strings.TrimPrefix(r.repoURL, "https://"), "/", 3); len(ss) == 3 {
		params.Owner, params.Repo = ss[1], ss[2]
	}

	hash := headCommit.Hash.String()
	if c != nil && c.RunID != 0 {
		if c.EventName != "pull_request" {
			return nil
		}

		hash = c.SHA
		params.Owner, params.Repo = c.Repo()
		if params.PRNumber == 0 { // It may happen depending on how the job is triggered.
			s, _, ok := strings.Cut(c.RefName, "/")
			if !ok {
				return errors.Errorf("parsing ref name %q", c.RefName)
			}

			params.PRNumber, err = strconv.Atoi(s)
			if err != nil {
				return errors.WithStack(err)
			}
		}
	}

	body := CommentBodyParams{
		Module:           module,
		RepoURL:          r.repoURL,
		Hash:             hash,
		Total:            total,
		Patch:            patch,
		OldTotal:         old,
		HideTotal:        r.hideTotal,
		HidePatch:        r.hidePatch,
		HideHowToImprove: r.hideHowToImprove,
		Changes:          changes,
	}

	err = CreateComment(ctx, r.action, params, body, r.templatePath)
	if err != nil {
		return err
	}

	return nil
}
