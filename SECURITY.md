# Security Policy

## Reporting a Vulnerability

If you find a security vulnerability, please report it responsibly:

1. **Do not** open a public GitHub issue
2. Use [GitHub Security Advisories](https://github.com/Mike-7777777/cx/security/advisories/new) to report privately
3. Include steps to reproduce and potential impact

I will respond within 48 hours and work with you on a fix before public disclosure.

## Scope

cx reads and writes local files only:
- Claude Code config directories (`~/.claude/`, `~/.claude-*/`)
- Registry file (`~/.cx.json`)
- Rate cache files (`rate-cache.json`)

cx does **not** make network requests, store credentials, or access external APIs.

## Supported Versions

Only the latest release receives security updates.
