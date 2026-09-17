# obsolete host configuration seams

problem: the only file loader passes an `os.File` through a private file
interface and a one-caller function. decoding is another one-caller wrapper.
these seams obscure the linear file admission and decoding path.

evidence: `hostConfigFile` has one production supplier; `loadOpenedHostConfig`
and `decodeConfig` each have one caller in `internal/hostconfig`. the behavioral
harnesses that used replacement files are retired.

resolved when: keep one explicit file-load path and strict decoder, preserving
file flags, size bounds, close-error handling, null/presence semantics and
platform validation. characterize actual temporary-file admission before and
after. retain the meaningful `stringField` presence contract.
