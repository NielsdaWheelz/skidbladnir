# duplicated release tag admission

problem: `render-release-notes` duplicates the version parser without its final
android version-code bounds. it accepts tags such as `v0.0.0`, `v0.0.1` and
`v2100.0.1` that `check-release --android-version` rejects.

impact: standalone rendering disagrees with the release owner. the current
`release` caller already validates the tag, so this does not establish a broken
published artifact. `check-release-source` also computes the same code twice.

resolved when: reuse the existing version admission command and its returned
code. exercise real rendering with synthetic release metadata, preserving valid
notes byte-for-byte and rejecting invalid tags without signing or publication.
