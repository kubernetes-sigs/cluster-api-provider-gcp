# AI Usage Policy

The Cluster API Provider GCP (CAPG) project has rules for AI usage.
The project acknowledges the benefits from thoughtful AI-assisted development,
but contributors must maintain high standards for code quality, security, and collaboration.

## Why do we have the policy
This policy is not to discourage the use of AI and should not be seen as anti-AI.
Instead, it's needed to ensure that the project standards are adhered to and
that contributors fully understand their contributions.

All issues and PRs require time from the human maintainers, reviewers and other contributors.
This policy exists to protect the valuable and often limited time of these humans from poor quality changes.

## Principles

- **Human accountability:** Contributors are responsible for all submitted
  changes. Understand the work well enough to explain, maintain, and support
  it.
- **Quality:** AI-assisted work must meet the same quality, testing, security,
  and review standards as any other contribution.
- **Transparency:** Disclose substantive AI assistance so reviewers understand
  how the contribution was produced and validated.

## Rules

- **You must declare AI usage**. When submitting a PR that contains AI assisted work the use of AI this must be stated.
Listing AI tooling as a co-author, co-signing commits using an AI tool, or using the assisted-by, co-developed or similar commit trailer is not allowed.
- **AI must not be used for PR descriptions**. A key indicator to the reviewers and maintainers that you understand
your contribution is a PR description written by yourself with your understanding.
If a PR description looks to be AI generated it will be closed.
Moreover PR descriptions must exactly follow the prescribed .github/PULL_REQUEST_TEMPLATE.md.
- **AI must not be used to create issues**. Like with PR descriptions issues
must be created by humans based on their knowledge or experiences with features or issues.
Moreover PR descriptions must exactly follow the prescribed .github/ISSUE_TEMPLATE/*.
Don't use AI to respond PR comments. Reviewers want to engage directly with you, not with generated responses.
If you do not engage directly with reviewers, the PR will be closed.

## Best Practices

### ✅ Recommended Uses
- Generating boilerplate code and common patterns
- Creating comprehensive test suites
- Refactoring existing code for clarity
- Generating utility functions and helpers
- Explaining existing code patterns

### ❌ Avoid AI For
- API Version bumps
- CAPI Contract changes
- Complex logic without thorough review
- Security critical authentication/authorization code
- Code you don’t fully understand
- Large architectural changes
