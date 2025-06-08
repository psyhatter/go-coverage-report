# Go Coverage Report GitHub Action

This GitHub Action provides an easy way to handle Go code coverage reports, both for pull requests and pushes. It can
post comments with detailed code coverage information, make comparisons with previous coverage, and store coverage data
using `git-notes`. This action is useful for Go developers aiming to maintain and improve test coverage in their
projects.

## Features

* Parses and displays the current total and patch code coverage.
* Compares current coverage with previous coverage to highlight changes.
* Posts detailed comments on pull requests with coverage information.
* Skips generated files (e.g., protobuf, swagger, mocks) during analysis.
* Allows custom comment templates for flexibility.
* Saves coverage data using `git-notes` during pushes (optional).

## Quick Start

Add the following step to your workflow file:

```yaml
- uses: psyhatter/go-coverage-report@v0.0.1
```

## Inputs

Below is a list of all available input parameters and their descriptions. Default values are provided where applicable.

| Input Name              | Description                                                                                                                     | Default value     |
|:-----------------------:|:-------------------------------------------------------------------------------------------------------------------------------:|:-----------------:|
| `coverage_file`         | File containing the Go coverage data.<br/>Example of generation:<br/>`go test -coverprofile=coverage.out -coverpkg=./... ./...` | coverage.out      |
| `prev_coverage_file`    | File containing the Go coverage data from before changes.<br/>Used for comparison. If not provided, will use `prev_coverage`.   | prev-coverage.out |
| `prev_coverage`         | Previous coverage percentage for comparison.                                                                                    | 0.0               |
| `skip_generated`        | Skips analyzing generated files (e.g., proto, swagger, mocks).                                                                  | true              |
| `skip_package_main`     | Skips analyzing package main files.                                                                                             | true              |
| `github_token`          | GitHub access token to post comments.<br/>If not provided, commenting will be skipped.                                          | ${{github.token}} |
| `hide_total`            | Hides the total coverage section in comments.                                                                                   | false             |
| `hide_patch`            | Hides the patch coverage section in comments.                                                                                   | false             |
| `hide_how_to_improve`   | Hides the "How to Improve Coverage" section in comments.                                                                        | false             |
| `changes_limit`         | Limits the number of changes displayed in the "How to Improve Coverage" section.                                                | 100               |
| `template_path`         | Custom template file path for comments.                                                                                         |                   |
| `use_git_notes`         | Enable storing coverage files in `git-notes`.                                                                                   | false             |
| `version`               | The version of the reporter utility to use.                                                                                     | v0.0.1            |
| `cover_ignore_filepath` | Path to the file containing patterns for files to ignore during coverage analysis.                                              | .coverignore      |

## Outputs

The action provides the following outputs:

| Output Name  | Description                                                |
|--------------|------------------------------------------------------------|
| `total`      | Current total percentage of test coverage.                 |
| `patch`      | Coverage percentage of lines modified by the pull request. |
| `prev_total` | Previous total test coverage percentage.                   |
| `diff`       | Difference between current total and previous total.       |

## Sticky Comment

Sends a comment that visualizing the coverage of the code with tests.
Here is an example:

#### :bar_chart: Code Coverage Report

|            Metric           | Value |     Visualization      |
|-----------------------------|-------|------------------------|
| **Total Coverage**          | 61.9% | 🟨🟨🟨🟨🟨🟨⬜⬜⬜⬜ |
| **Coverage for PR Changes** | 80.0% | 🟩🟩🟩🟩🟩🟩🟩🟩⬜⬜ |

#### Notes:

- **Total Coverage** - coverage indicator within the entire code.
- **Coverage for PR Changes** - coverage of changes made within this PR only.

<details>
    <summary><b>Let's improve the coverage of tests!</b> Click here to find out how</summary>
    <p>
    <details>
        <summary>path/to/your/file_1.go</summary>
        https://github.com/psyhatter/go-coverage-report/blob/16f4d12f6a8ccc08170a880fdb39181ad0a19f43/path/to/your/file_1.go#L7-L10
    </details>
    <details>
        <summary>path/to/your/file_2.go</summary>
        https://github.com/psyhatter/go-coverage-report/blob/16f4d12f6a8ccc08170a880fdb39181ad0a19f43/path/to/your/file_2.go#L7-L10
    </details>
    </p>
</details>

## Usage

Here's how you can configure this action in your workflow file:

## FAQs

### 1. How do I customize the posted comments?

You can use the `hide_total`, `hide_patch` or `hide_how_to_improve` features.
Or you can use the `template_path` input to provide a custom template for comments.
This enables you to structure the report as needed.

### 2. What does the `use_git_notes` option do?

When enabled, the action uses `git-notes` to store the coverage file.
This can be useful to persist coverage information across branches or commits.

### 3. What happens when `github_token` is not provided?

If no `github_token` is provided, the action will work but will not post comments on pull requests.

## License

This project is licensed under the [MIT License](LICENSE).

# TODOS

- Test git notes flow
