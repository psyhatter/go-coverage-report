package reporter

import (
	"bufio"
	"cmp"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unique"

	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/pkg/errors"
	"github.com/sethvargo/go-githubactions"
)

func ParseCoverageFile(
	coverage io.Reader,
	module string,
	commit *object.Commit,
	ignore []*regexp.Regexp,
	skipGenerated, skipMain bool,
) (statements map[Filename][]CoverageBlock, err error) {
	defer func() {
		if closer, ok := coverage.(io.Closer); ok {
			_ = closer.Close()
		}
	}()

	rr := bufio.NewScanner(coverage)
	if !rr.Scan() { // Skip header.
		return statements, errors.WithStack(rr.Err())
	}

	skipping := make(map[Filename]bool)
	handled := make(map[string]struct{})
	statements = make(map[Filename][]CoverageBlock)
Loop:
	for rr.Scan() {
		line := rr.Bytes()

		if _, ok := handled[string(line)]; ok {
			continue
		}
		handled[string(line)] = struct{}{}

		filename, c, err := parseLine(string(line))
		if err != nil {
			return statements, err
		}

		path := unique.Make(strings.TrimPrefix(filename, module+"/"))
		skip, found := skipping[path]
		if !found {
			for _, re := range ignore {
				if re.MatchString(filename) {
					skipping[path] = true
					continue Loop
				}
			}

			skip, err = ShouldSkip(commit, filename, skipGenerated, skipMain)
			if err != nil {
				return statements, errors.WithStack(err)
			}
			skipping[path] = skip
		}
		if skip {
			continue
		}

		stmts, ok := statements[path]
		if !ok {
			statements[path] = []CoverageBlock{c}
			continue
		}

		i, found := slices.BinarySearchFunc(stmts, c, func(a CoverageBlock, b CoverageBlock) int {
			if a.StartLine == b.StartLine {
				return cmp.Compare(a.StartCol, b.StartCol)
			}
			return cmp.Compare(a.StartLine, b.StartLine)
		})
		switch {
		case !found:
			statements[path] = slices.Insert(stmts, i, c)
		case c.Covered:
			stmts[i].Covered = true
		}
	}

	if err = rr.Err(); err != nil {
		return statements, errors.WithStack(err)
	}

	for filename, changes := range statements {
		var n int
		for _, change := range changes {
			if n > 0 {
				if prev := &changes[n-1]; prev.Covered == change.Covered && prev.EndLine+1 >= change.StartLine {
					prev.NumStmt, prev.EndLine, prev.EndCol = prev.NumStmt+change.NumStmt, change.EndLine, change.EndCol
					continue
				}
			}

			changes[n], n = change, n+1
		}

		statements[filename] = changes[:n]
	}

	return statements, nil
}

type CoverageBlock struct {
	NumStmt, StartLine, StartCol, EndLine, EndCol int
	Covered                                       bool
}

// parseLine parses a line from a coverage file.
// It is equivalent to the regex
// ^(.+):([0-9]+)\.([0-9]+),([0-9]+)\.([0-9]+) ([0-9]+) ([0-9]+)$
//
// However, it is much faster: https://golang.org/cl/179377
func parseLine(l string) (filename string, c CoverageBlock, err error) {
	count, end, err := seekBack(l, ' ', len(l), "Count")
	if err != nil {
		return filename, c, err
	}
	c.Covered = count > 0
	c.NumStmt, end, err = seekBack(l, ' ', end, "NumStmt")
	if err != nil {
		return filename, c, err
	}
	c.EndCol, end, err = seekBack(l, '.', end, "EndCol")
	if err != nil {
		return filename, c, err
	}
	c.EndLine, end, err = seekBack(l, ',', end, "EndLine")
	if err != nil {
		return filename, c, err
	}
	c.StartCol, end, err = seekBack(l, '.', end, "StartCol")
	if err != nil {
		return filename, c, err
	}
	c.StartLine, end, err = seekBack(l, ':', end, "StartLine")
	if err != nil {
		return filename, c, err
	}
	filename = l[:end]
	if filename == "" {
		return filename, c, errors.New("a FileName cannot be blank")
	}
	return filename, c, nil
}

// seekBack searches backwards from end to find sep in l, then returns the
// value between sep and end as an integer.
// If seekBack fails, the returned error will reference what.
func seekBack(l string, sep byte, end int, what string) (value int, nextSep int, err error) {
	// Since we're seeking backwards and we know only ASCII is legal for these values,
	// we can ignore the possibility of non-ASCII characters.
	for start := end - 1; start >= 0; start-- {
		if l[start] == sep {
			value, err = strconv.Atoi(l[start+1 : end])
			if err != nil {
				return 0, 0, errors.Errorf("parsing %q: %v", what, err)
			}
			if value < 0 {
				return 0, 0, errors.Errorf("negative values are not allowed for %s, found %d", what, value)
			}
			return value, start, nil
		}
	}
	return 0, 0, errors.Errorf("couldn't find a %s before %s", string(sep), what)
}

func NewRange(start, end int) Range {
	if start > end {
		start, end = end, start
	}
	return Range{Start: start, End: end}
}

type Range struct{ Start, End int }

func TotalCoverage(statements map[Filename][]CoverageBlock) float64 {
	if len(statements) == 0 {
		return 0
	}

	var total, covered int
	for _, blocks := range statements {
		for _, block := range blocks {
			total += block.NumStmt
			if block.Covered {
				covered += block.NumStmt
			}
		}
	}

	return float64(covered) / float64(total) * 100
}

func PatchCoverage(changes map[Filename][]CodeBlock) float64 {
	if len(changes) == 0 {
		return 100
	}

	total, covered := 0, 0
	for _, blocks := range changes {
		for _, block := range blocks {
			if block.HasStatement {
				lines := block.End - block.Start + 1
				total += lines
				if block.Covered {
					covered += lines
				}
			}
		}
	}

	return float64(covered) / float64(total) * 100
}

func Merge(changes map[Filename][]CodeBlock, coverage map[Filename][]CoverageBlock) map[Filename][]CodeBlock {
	result := make(map[Filename][]CodeBlock, len(changes))
	for filename, blocks := range changes {
		var mergedBlocks []CodeBlock
		coverageBlocks := coverage[filename]

		for _, block := range blocks {
			mergedBlocks = append(mergedBlocks, mergeBlockWithCoverage(block, coverageBlocks)...)
		}

		result[filename] = mergedBlocks
	}
	return result
}

func mergeBlockWithCoverage(block CodeBlock, coverageBlocks []CoverageBlock) (result []CodeBlock) {
	for startLine := block.Start; startLine <= block.End; {
		idx, found := slices.BinarySearchFunc(coverageBlocks, startLine, func(b CoverageBlock, line int) int {
			if b.StartLine > line {
				return 1
			}
			if b.EndLine < line {
				return -1
			}
			return 0
		})

		if found {
			covered := coverageBlocks[idx]
			c := CodeBlock{
				HasStatement: true,
				Covered:      covered.Covered,
				Range:        NewRange(startLine, min(covered.EndLine, block.End)),
			}

			result, startLine = append(result, c), c.Range.End+1
			continue
		}

		idx, _ = slices.BinarySearchFunc(coverageBlocks, startLine, func(b CoverageBlock, line int) int {
			return cmp.Compare(b.StartLine, line)
		})

		nextCovStart := block.End + 1
		if idx < len(coverageBlocks) && coverageBlocks[idx].StartLine <= block.End {
			nextCovStart = coverageBlocks[idx].StartLine
		}

		endLine := min(nextCovStart-1, block.End)
		result, startLine = append(result, CodeBlock{Range: NewRange(startLine, endLine)}), endLine+1
	}

	return result
}

func OldCoverage(
	action *githubactions.Action,
	root, filename string,
	commit *object.Commit,
	ignore []*regexp.Regexp,
	skipGenerated, skipMain bool,
) (float64, error) {
	if filename == "" {
		return 0, nil
	}

	path := filepath.Join(root, filename)

	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			action.Warningf("old coverage file %q doesn't exist, skipping", path)
			return 0, nil
		}
		return 0, errors.WithStack(err)
	}

	oldStatements, err := ParseCoverageFile(file, root, commit, ignore, skipGenerated, skipMain)
	if err != nil {
		return 0, err
	}

	return TotalCoverage(oldStatements), nil
}

func UncoveredChanges(changes map[Filename][]CodeBlock, limit int) map[Filename][]CodeBlock {
	if limit < 0 {
		return changes
	}

	if limit == 0 {
		return nil
	}

	var changesCount int
	for filename, blocks := range changes {
		blocks = slices.DeleteFunc(blocks, func(b CodeBlock) bool { return !b.HasStatement || b.Covered })
		if len(blocks) == 0 {
			delete(changes, filename)
			continue
		}

		changes[filename], changesCount = blocks, changesCount+len(blocks)
	}

	if changesCount <= limit {
		return changes
	}

	type Uncovered struct {
		Filename
		CodeBlock
	}

	uncovered := make([]Uncovered, 0, changesCount)
	for filename, blocks := range changes {
		for _, block := range blocks {
			uncovered = append(uncovered, Uncovered{Filename: filename, CodeBlock: block})
		}
	}

	slices.SortStableFunc(
		uncovered,
		func(a, b Uncovered) int {
			return cmp.Compare(a.End-a.Start, b.End-b.Start)
		},
	)

	changes = make(map[Filename][]CodeBlock, min(len(changes), limit))
	for _, u := range uncovered[:limit] {
		changes[u.Filename] = append(changes[u.Filename], u.CodeBlock)
	}

	return changes
}
