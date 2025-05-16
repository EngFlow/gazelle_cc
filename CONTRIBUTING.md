# Contributing to gazelle_cc

gazelle_cc is an open source project, licensed with the Apache 2.0 license. We welcome your contributions!

## File an issue

To report a bug or suggest a new feature, please file an issue. We like to discuss and confirm planned solutions before reviewing code. This is especially important before sending a PR that adds a feature or changes the user interface (for example, adding a new directive).

No issue is necessary for small PRs, such as those improving documentation or fixing bugs with a clear solution.

## Submit a PR

1. Fork the repository.
1. Write your change. Please add tests and update documentation when appropriate.
1. Developer Certificate of Origin (DCO): sign off all commits.

    The sign-off is a simple line at the end of the commit message that certifies that you wrote the patch or otherwise have the write to pass it on as an open-source patch. See https://developercertificate.org/ for details.

    You can manually add the following line to your commit messages:

    ```
    Signed-off-by: J. Random Bazeler <jrbazel@example.com>
    ```

    You can sign off your match automatically using `git commit -s` or `git commit --signoff`. To make this more convenient, run the commands below to add `git amend` and `git c` aliases that include this flag:

    ```bash
    git config --add alias.amend "commit --signoff --amend" && \
    git config --add alias.c "commit --signoff"
    ```

    Use your real name and email address. We do not accept anonymous contributions.

    All commits on the PR branch must pass the DCO check. If your PR fails the DCO check, you can squash the commit history into a single commit, append the DCO sign-off to the commit message, then force push. For example

    ```bash
    git rebase --interactive HEAD~3
    # squash 3 commits, add Signed-off-by to commit message
    git push --force
    ```

    Avoid rewriting history if you can. It complicates the review process.

1. Create your PR.

    Draft PRs may not be reviewed, so do not create your PR as a draft if you want prompt reviews.

    When opening a PR, you are expected to actively work on until it is merged or closed. We reserve the right to close PRs that are not making progress. PRs that are closed due to lack of activity can be reopened later.

    The PR title should be descriptive and ideally starts with a topic or package name, followed by a colon and more information, for example,

    ```
    language/cc: support Plan 9 style tests.
    docs: convert documentation to LaTeX
    ```

    The PR description should have details on what the PR does. If it fixes an existing issue, it should include "Fixes #XXX".

    The PR description will be used as the commit message when the PR is merged. Update this field if your PR diverges during review.

    If your PR is co-authored or based on an earlier PR from another contributor, attribute them with `Co-authored-by: name <name@example.com>`. See GitHub's multiple author guidance for details.

    When adding new code, add tests covering it.

    PRs are only merged if all tests pass.
