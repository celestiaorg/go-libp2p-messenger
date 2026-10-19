# CLAUDE.md

## Security PRs

- For PRs that resolve Hacken bug bounty reports, do not include details about the bug in the PR description. Instead, link to a Linear issue that contains more details on the bug and the link to the Hacken bug bounty report.

## Pull Requests

- Title format: `type(scope)!: [level] subject` (conventional commit). The subject starts with exactly one of `[high]`, `[medium]`, or `[low]`. CI checks this.
  - `[high]`: Deep review for message framing/serialization, stream handling/concurrency, peer management, security, or other changes with significant risk.
  - `[medium]`: Standard review for code changes with limited impact and well-understood behavior.
  - `[low]`: Focused review for straightforward docs, tests, or tooling changes with low risk.
- Choose the higher level when unsure. A small diff can still need deep review.
- Include the same tag in the PR description with a brief reason, e.g. `Review level: [low] — documentation only.`
