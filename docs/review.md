# Review rules

Follow [CONTRIBUTING](../CONTRIBUTING.md) and run `make check` before a PR. CI additionally runs the race detector and golangci-lint. No separate automated pre-PR review tool is configured.

For graphical identity, kit or future frontend changes, read [the brand reference](reference/brand.md) and [serverku-brand skill](../.claude/skills/serverku-brand/SKILL.md).

- Run every applicable guard at each commit: `make brand-check`; for kit changes also `make brand-verify` and `make brand-rebuild`. Check `git diff --check`.
- Confirm palette roles as well as values. Review every new unchecked entry's name, scope and reason.
- Open all changed guide pages, refresh the PDF review hash only after inspection, and inspect after goldens against the preserved baseline. Reject unrelated golden changes, especially dates.
- Ensure terminal appearance is preserved. No current frontend/typecheck suite exists; future surfaces must run their own tests, type checks and builds.
- Check graphical wordmark weight, adjacent UI-label typography, contrast, clear space and placed-image scope. Future frontends need dev/production base-path checks, visible mask fallbacks and appropriate Flutter variable-font settings.
- Request an independent reviewer, fix actionable findings in follow-up commits, rebase on latest master, and check CI on the final pushed head.

Review documentation claims against the implemented guard. It is a palette/integrity scanner, not a general parser or accessibility checker. Historical evidence is not current product behavior.
