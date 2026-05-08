# Issue Tracker

Issues and PRDs live in GitHub Issues for `Hector-Ha/InstantRepo`.

Use `gh` CLI from repo root. Let `gh` infer repo from `git remote`.

## Commands

- Create issue: `gh issue create --title "..." --body "..."`
- Read issue: `gh issue view <number> --comments`
- List issues: `gh issue list --state open --json number,title,body,labels,comments`
- Comment: `gh issue comment <number> --body "..."`
- Add label: `gh issue edit <number> --add-label "..."`
- Remove label: `gh issue edit <number> --remove-label "..."`
- Close: `gh issue close <number> --comment "..."`

## Skill Rules

- When skill says "publish to issue tracker", create GitHub issue.
- When skill says "fetch ticket", run `gh issue view <number> --comments`.
- Use heredoc for long issue bodies.
- Do not invent another tracker unless this file changes.
