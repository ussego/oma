# Security

## Trust boundary

oma plugins are plain JavaScript that runs inside `omarchy-shell` with your
session's privileges. **Only install plugins you trust** — a plugin has full
access to your session, the same as any program you run. oma's job is to make
building such plugins pleasant; it does not sandbox them.

## Reporting vulnerabilities

There is no private reporting channel yet. Report vulnerabilities by opening
a GitHub issue at https://github.com/ussego/oma/issues — mark the title with
`[security]` and include the affected version, a minimal repro, and the
impact. Do not publish exploit details before a fix is available.

If you find a vulnerability in Omarchy or Quickshell itself, report it to
those projects instead.
