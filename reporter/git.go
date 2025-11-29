package reporter

import (
	"context"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
	"unique"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/pkg/diff/edit"
	"github.com/pkg/diff/myers"
	"github.com/pkg/errors"
	"github.com/sethvargo/go-githubactions"
)

func GetHeadCommit(repo *git.Repository) (*object.Commit, error) {
	headRef, err := repo.Head()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	head, err := repo.CommitObject(headRef.Hash())

	return head, errors.WithStack(err)
}

func GetBranchCommit(repo *git.Repository, branchName string) (*object.Commit, error) {
	branchRef, err := repo.Reference(plumbing.NewRemoteReferenceName("origin", branchName), true)
	if err != nil {
		return nil, errors.Wrapf(err, "getting reference %q", branchName)
	}

	commit, err := repo.CommitObject(branchRef.Hash())

	return commit, errors.WithStack(err)
}

func GetCommonAncestor(a, b *object.Commit) (*object.Commit, error) {
	commonAncestors, err := a.MergeBase(b)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if len(commonAncestors) == 0 {
		return nil, errors.Errorf("%s and %s have no common ancestors", a.Hash.String(), b.Hash.String())
	}

	return commonAncestors[0], nil
}

type Filename = unique.Handle[string]

func GetChanges(
	ctx context.Context,
	action *githubactions.Action,
	module string,
	branchCommit, headCommit *object.Commit,
	ignore []*regexp.Regexp,
	skipGenerated bool,
	skipMain bool,
) (changed map[Filename][]CodeBlock, err error) {
	commonAncestor, err := GetCommonAncestor(branchCommit, headCommit)
	if err != nil {
		return nil, err
	}

	headTree, err := headCommit.Tree()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	branchTree, err := commonAncestor.Tree()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	changes, err := object.DiffTreeWithOptions(ctx, branchTree, headTree, &object.DiffTreeOptions{DetectRenames: true})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	action.Group("Parsing PR changes")
	defer action.EndGroup()

	m := make(map[Filename][]CodeBlock, len(changes))
	for _, change := range changes {
		if !strings.HasSuffix(change.To.Name, ".go") {
			action.Infof("skipping non-Go file %q", change.To.Name)
			continue
		}

		if strings.HasSuffix(change.To.Name, "_test.go") {
			action.Infof("skipping test file %q", change.To.Name)
			continue
		}

		for _, r := range ignore {
			if r.MatchString(change.To.Name) {
				action.Infof("skipping  %q file, ignored by rule %q", change.To.Name, r)
				continue
			}
		}

		c, err := GitDiff(action, change, skipGenerated, skipMain)
		if err != nil {
			return nil, err
		}

		m[unique.Make(change.To.Name)] = c
	}

	return m, nil
}

// GitDiff computes the changes between two file versions provided in the given
// object.Change and returns a slice of CodeBlock.
func GitDiff(action *githubactions.Action, c *object.Change, skipGenerated, skipMain bool) ([]CodeBlock, error) {
	from, to, err := c.Files()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// Skip removed files.
	if to == nil {
		return nil, nil
	}

	// Skip binary files.
	if isBin, err := to.IsBinary(); isBin || err != nil {
		action.Infof("skipping binary file %q", to.Name)
		return nil, errors.WithStack(err)
	}

	toContent, err := to.Contents()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if skipMain {
		file, err := parser.ParseFile(token.NewFileSet(), to.Name, toContent, parser.PackageClauseOnly)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		if file.Name.Name == "main" {
			action.Infof("skipping package main file %q", to.Name)
			return nil, nil
		}
	}

	if skipGenerated {
		firstLine, _, _ := strings.Cut(toContent, "\n")
		if IsGenerated(firstLine) {
			action.Infof("skipping generated file %q", to.Name)
			return nil, nil
		}
	}

	var fromContent string
	if from != nil {
		if isBin, err := from.IsBinary(); isBin || err != nil {
			action.Infof("skipping file that was binary %q", from.Name)
			return nil, errors.WithStack(err)
		}

		fromContent, err = from.Contents()
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}

	return DiffStrings(fromContent, toContent), nil
}

type CodeBlock struct {
	Range
	HasStatement, Covered bool
}

func DiffStrings(fromContent, toContent string) []CodeBlock {
	a := strings.Split(fromContent, "\n")
	b := strings.Split(toContent, "\n")
	ab := stringsPair{a: make([]unique.Handle[string], len(a)), b: make([]unique.Handle[string], len(b))}

	for i, s := range a {
		ab.a[i] = unique.Make(s)
	}
	for i, s := range b {
		ab.b[i] = unique.Make(s)
	}

	// TODO: This implementation of the algorithm is slightly different from the one used in git diff.
	s := myers.Diff(nil, ab)

	var inserts int
	for _, seg := range s.Ranges {
		if seg.Op() == edit.Ins {
			inserts++
		}
	}

	result := make([]CodeBlock, 0, inserts)
	for _, seg := range s.Ranges {
		switch seg.Op() {
		case edit.Ins:
			result = append(result, CodeBlock{Range: Range{Start: seg.LowB + 1, End: seg.HighB}})
		}
	}

	return result
}

type stringsPair struct{ a, b []unique.Handle[string] }

func (ab stringsPair) LenA() int             { return len(ab.a) }
func (ab stringsPair) LenB() int             { return len(ab.b) }
func (ab stringsPair) Equal(ai, bi int) bool { return ab.a[ai] == ab.b[bi] }
