# group labels admit invisible characters

problem: `group.Parse` accepts invisible code points in a label. `singleLine`
prints some as spaces (u+200b, u+2060, u+feff, u+00ad) and passes others through
(u+034f, u+fe0f, u+3164). distinct labels can print identically, a label of only
u+200b prints blank, and `all` followed by u+034f prints as a bare `all`.

impact: two groups can read alike wherever labels print: the desktop strip,
group headings, details and phone pickers. named labels now display without a
prefix by request; removing the former desktop collision prefix leaves this
independent display-safety issue unresolved.

evidence: 2026-09-29, `group.Parse` returned `"a\u200bb"`, `"\u200ball"`,
`"\u2060all"`, `"\u200b"` and `"agents\u00ad"` unchanged. the desktop strip drew
`a b  a b  group:  all  group:  all` for `a b`, `"a\u200bb"`, `"\u200ball"` and
`"\u2060all"`, and a bare `all` or `agents` for `"all\u034f"`, `"all\ufe0f"` and
`"agents\u034f"`.

resolved when: the [label rules](../groups.md#3-labels-equality-and-ordering)
reject invisible code points, or define how they print, and parse, the gateway
and the phone agree.
