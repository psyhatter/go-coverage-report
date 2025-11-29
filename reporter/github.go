package reporter

import (
	"bytes"
	"cmp"
	"context"
	_ "embed"
	"fmt"
	"math"
	"net/url"
	"os"
	"slices"
	"strings"
	"text/template"

	"github.com/google/go-github/v79/github"
	"github.com/pkg/errors"
	"github.com/sethvargo/go-githubactions"
)

//go:embed comment.gohtml
var DefaultTemplate string

type GitHubParams struct {
	Token, Owner, Repo string
	PRNumber           int
}

type CommentBodyParams struct {
	Module, Hash, RepoURL                  string
	Total, Patch, OldTotal                 float64
	HideTotal, HidePatch, HideHowToImprove bool
	Changes                                map[Filename][]CodeBlock
}

const BodyPrefix = "<!-- go-coverage-report -->"

func ValidateParams(params GitHubParams, body CommentBodyParams) error {
	switch {
	default:
		return nil
	case params.Token == "":
		return errors.New("github token is not set")
	case params.Owner == "":
		return errors.New("github owner is not set")
	case params.Repo == "":
		return errors.New("github repo is not set")
	case params.PRNumber == 0:
		return errors.New("github pr number is not set")
	case body.Module == "":
		return errors.New("go module is not set")
	case body.Hash == "":
		return errors.New("commit hash is not set")
	case body.RepoURL == "":
		return errors.New("repo url is not set")
	}
}

func CreateComment(
	ctx context.Context,
	action *githubactions.Action,
	params GitHubParams,
	body CommentBodyParams,
	templatePath string,
) error {
	err := ValidateParams(params, body)
	if err != nil {
		action.Warningf("%v, skip comment posting", err)
		return nil
	}

	client := github.NewClient(nil).WithAuthToken(params.Token)

	templ := DefaultTemplate
	if templatePath != "" {
		b, err := os.ReadFile(templatePath)
		if err != nil {
			return errors.WithStack(err)
		}

		templ = string(b)
	}

	funcMap := template.FuncMap{
		"link": func() func(file Filename, block CodeBlock) (string, error) {
			if !strings.HasPrefix(body.RepoURL, "http") {
				body.RepoURL = "https://" + body.RepoURL
			}

			link, err := url.Parse(body.RepoURL)
			if err != nil {
				return func(file Filename, block CodeBlock) (string, error) {
					return "", errors.WithStack(err)
				}
			}

			link = link.JoinPath("blob", body.Hash)

			return func(file Filename, block CodeBlock) (string, error) {
				link := link.JoinPath(file.Value())
				link.Fragment = fmt.Sprintf("L%d-L%d", block.Start, block.End)
				return link.String(), nil
			}
		}(),
		"progressBar": func(f float64) string {
			a := int(math.Round(f / 10))
			b := 10 - a
			return strings.Repeat(getColoredBlock(f), a) + strings.Repeat("⬜", b)
		},
		"totalDiff": func() string {
			if body.OldTotal == 0 || body.Total == body.OldTotal {
				return ""
			}
			diff := fmt.Sprintf(`%+0.1f%%`, body.Total-body.OldTotal)
			if body.Total > body.OldTotal {
				return ` ⬆ ` + diff
			}
			return ` ⬇ ` + diff
		},
		"sortedFilenames": func() []Filename {
			filenames := make([]Filename, 0, len(body.Changes))
			for filename := range body.Changes {
				filenames = append(filenames, filename)
			}

			slices.SortFunc(filenames, func(a, b Filename) int {
				return cmp.Compare(a.Value(), b.Value())
			})

			return filenames
		},
		"trimPrefix": strings.TrimPrefix,
	}

	tmpl, err := template.New("comment_body").Funcs(funcMap).Parse(templ)
	if err != nil {
		return errors.WithStack(err)
	}

	var buf bytes.Buffer
	if err = tmpl.Execute(&buf, body); err != nil {
		return errors.WithStack(err)
	}

	if len(bytes.TrimSpace(buf.Bytes())) == 0 {
		action.Warningf("comment body is empty, skip comment posting")
		return nil
	}

	in := github.IssueComment{
		Body: github.Ptr(BodyPrefix + "\n" + buf.String()),
	}

	for i := range 10 {
		opts := github.IssueListCommentsOptions{ListOptions: github.ListOptions{Page: i + 1, PerPage: 100}}
		comments, _, err := client.Issues.ListComments(ctx, params.Owner, params.Repo, params.PRNumber, &opts)
		if err != nil {
			return errors.WithStack(err)
		}

		if len(comments) == 0 {
			break
		}

		for _, comment := range comments {
			if strings.HasPrefix(comment.GetBody(), BodyPrefix) {
				_, _, err = client.Issues.EditComment(ctx, params.Owner, params.Repo, comment.GetID(), &in)
				return errors.WithStack(err)
			}
		}
	}

	_, _, err = client.Issues.CreateComment(ctx, params.Owner, params.Repo, params.PRNumber, &in)

	return errors.WithStack(err)
}

func getColoredBlock(f float64) string {
	switch {
	case f >= 75:
		return "🟩"
	case f >= 50:
		return "🟨"
	case f >= 25:
		return "🟧"
	default:
		return "🟥"
	}
}
