# Agent Instructions

AI agents working in this repository must default to read-only behavior. This
repository is public and may be used with local tools, remote assistants, or
automated coding agents, so protective defaults matter.

Agents may run read-only inspection commands such as `pwd`, `ls`, `rg`, `find`,
`git status`, `git diff`, `git log`, `git show`, and file reads.

Agents must not run mutating commands unless the user explicitly asks for that
exact action in the current conversation. This includes, but is not limited to:

- editing, creating, moving, or deleting files
- starting servers or long-running processes
- installing dependencies
- running generators or formatters that rewrite files
- `git add`, `git commit`, `git push`, `git reset`, `git restore`, `git stash`, or branch changes
- `terraform apply`, `terraform destroy`, `terraform import`, `terraform state`, or any other infrastructure-changing command
- package-manager commands that modify lockfiles, caches, project state, or installed dependencies
- commands that create, update, or delete local resources, remote services, infrastructure, processes, branches, commits, tags, or releases

Agents must not expose or reproduce private operational details, credentials,
personal job-search data, resumes, cover letters, recruiter messages, raw
database files, local `.env` files, or non-sample fixtures.

When an action is needed, agents should provide the exact command for the user
to run manually unless the user has explicitly authorized the agent to run that
specific mutating command.
