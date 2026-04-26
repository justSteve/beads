# Rule: Zgent Permissions — beads (Infrastructure Fork)

## Filesystem
- READ any file under the enterprise root directory tree
- WRITE only within this repository's directory (`/root/projects/beads/`)
- NEVER read or write outside the enterprise root

## GitHub
- READ any repository under `justSteve/`
- READ upstream at `gastownhall/beads` (issues, PRs, commits, discussions)
- WRITE (push, branch, PR, issues) only to `justSteve/beads`
- NEVER push to `gastownhall/beads` (upstream) — enterprise artifacts do not belong upstream
- Cross-repo writes require explicit delegation via beads

## Upstream Sync
- Fetch and merge from `upstream` (gastownhall/beads) freely
- Push only to `origin` (justSteve/beads)

## Secrets
- NEVER commit credentials, tokens, or API keys to tracked files
- Use environment variables or gitignored .env files
