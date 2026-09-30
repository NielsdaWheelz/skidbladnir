# exact tmux name and title comparisons

problem: `internal/tmux/client.go:formatLiteral` escapes format metacharacters
without a literal-expression boundary. tmux treats `##[` specially, so arbitrary
external session names or planned pane-title predicates may compare incorrectly.

impact: false stale-name rejection today is possible; automatic naming must not
adopt a title using an inexact predicate. no runtime reproduction is claimed.

evidence: the existing replacement-only helper and `format_expand1`,
`FORMAT_LITERAL`, and `format_unescape` in
[tmux 3.7 source](https://github.com/tmux/tmux/blob/3.7/format.c); the same relevant
mechanism exists in 3.4. [the naming plan](../automatic-session-names.md) owns the fix.

resolved when the shared owner uses an exact literal expression and temporary
isolated-tmux checks against a temporary option demonstrate comparison of
style/format-looking text, delimiters, quotes and newlines on both supported
host platforms. title ingestion itself rejects controls. no command
substitution may execute. remove this record after qualification.

blocker: live tmux reproduction/qualification needs current-turn authorization;
status `NOT_RUN`. no production file was changed during planning.
