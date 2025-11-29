package reporter

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCoverageFile(t *testing.T) {
	t.Parallel()

	module := "github.com/psyhatter/go-coverage-report"

	coverage := strings.NewReader(`mode: set
github.com/psyhatter/go-coverage-report/tests/coverable.go:3.16,4.12 1 0
github.com/psyhatter/go-coverage-report/tests/coverable.go:4.12,6.3 1 0
github.com/psyhatter/go-coverage-report/tests/coverable.go:9.2,9.12 1 0
github.com/psyhatter/go-coverage-report/tests/coverable.go:9.12,11.3 1 0
github.com/psyhatter/go-coverage-report/tests/coverable.go:16.2,16.11 1 0
github.com/psyhatter/go-coverage-report/tests/coverable.go:17.9,18.9 1 0
github.com/psyhatter/go-coverage-report/tests/coverable.go:19.9,20.9 1 0
github.com/psyhatter/go-coverage-report/tests/coverable.go:21.10,21.10 0 0
github.com/psyhatter/go-coverage-report/tests/coverable.go:3.16,4.12 1 1
github.com/psyhatter/go-coverage-report/tests/coverable.go:4.12,6.3 1 1
github.com/psyhatter/go-coverage-report/tests/coverable.go:9.2,9.12 1 1
github.com/psyhatter/go-coverage-report/tests/coverable.go:9.12,11.3 1 1
github.com/psyhatter/go-coverage-report/tests/coverable.go:16.2,16.11 1 0
github.com/psyhatter/go-coverage-report/tests/coverable.go:17.9,18.9 1 0
github.com/psyhatter/go-coverage-report/tests/coverable.go:19.9,20.9 1 0
github.com/psyhatter/go-coverage-report/tests/coverable.go:21.10,21.10 0 0
`)

	statements, err := ParseCoverageFile(coverage, module, nil, nil, false, false)
	require.NoError(t, err)

	expected := map[Filename][]CoverageBlock{
		"github.com/psyhatter/go-coverage-report/tests/coverable.go": {
			{
				NumStmt:   2,
				StartLine: 3,
				StartCol:  16,
				EndLine:   6,
				EndCol:    3,
				Covered:   true,
			},
			{
				NumStmt:   2,
				StartLine: 9,
				StartCol:  2,
				EndLine:   11,
				EndCol:    3,
				Covered:   true,
			},
			{
				NumStmt:   3,
				StartLine: 16,
				StartCol:  2,
				EndLine:   21,
				EndCol:    10,
				Covered:   false,
			},
		},
	}

	require.Equal(t, expected, statements)
}

func TestMerge(t *testing.T) {
	changes := map[Filename][]CodeBlock{
		"github.com/psyhatter/go-coverage-report/tests/coverable.go": {
			{
				Range: Range{
					Start: 6,
					End:   15,
				},
			},
			{
				Range: Range{
					Start: 18,
					End:   18,
				},
			},
		},
	}

	statements := map[Filename][]CoverageBlock{
		"github.com/psyhatter/go-coverage-report/tests/coverable.go": {
			{
				NumStmt:   2,
				StartLine: 3,
				StartCol:  16,
				EndLine:   6,
				EndCol:    3,
				Covered:   true,
			},
			{
				NumStmt:   2,
				StartLine: 9,
				StartCol:  2,
				EndLine:   11,
				EndCol:    3,
				Covered:   true,
			},
			{
				NumStmt:   3,
				StartLine: 16,
				StartCol:  2,
				EndLine:   21,
				EndCol:    10,
				Covered:   false,
			},
		},
	}

	expected := map[Filename][]CodeBlock{
		"github.com/psyhatter/go-coverage-report/tests/coverable.go": {
			{
				Range: Range{
					Start: 6,
					End:   6,
				},
				HasStatement: true,
				Covered:      true,
			},
			{
				Range: Range{
					Start: 7,
					End:   8,
				},
				HasStatement: false,
			},
			{
				Range: Range{
					Start: 9,
					End:   11,
				},
				HasStatement: true,
				Covered:      true,
			},
			{
				Range: Range{
					Start: 12,
					End:   15,
				},
				HasStatement: false,
			},
			{
				Range: Range{
					Start: 18,
					End:   18,
				},
				HasStatement: true,
				Covered:      false,
			},
		},
	}
	result := Merge(changes, statements)
	require.Equal(t, expected, result)
}
