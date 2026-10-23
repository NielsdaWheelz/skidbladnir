# group labels admit invisible characters

problem: `group.Parse` (`internal/group/group.go:36-52`) rejects controls, bidi
marks and non-ordinary spaces but accepts other invisible code points.
`attention.SingleLine` prints format characters as spaces (u+200b, u+2060,
u+feff, u+00ad) and passes others through (u+034f, u+fe0f, u+3164). distinct labels
can print identically, a label of only u+200b prints blank, and `all` followed by
u+034f prints as `all`.

impact: two groups can read alike wherever labels print: the desktop strip, group
headings, details and phone pickers.

evidence: 2026-09-29, `group.Parse` returned `"a​b"`, `"​all"`,
`"⁠all"`, `"​"` and `"agents­"` unchanged; the desktop strip drew
`a b` for both `a b` and `"a​b"`, and a bare `all` for `"all͏"` and
`"all️"`. the parse loop is unchanged at ba60841. 2026-10-22, a scratch
gateway at 8772799 answered `PUT …/group` with 204 for `"a​b"` and `"a͏b"`.

resolved when: `group.Parse` rejects invisible code points, or the host defines
how they print and every label display (desktop strip, phone pickers) follows it.
