# archive metadata check reads file contents

problem: `scripts/check-release` greps the complete uncompressed tar stream for
pax xattr records. matching ordinary file bytes are treated as metadata.

impact: a valid host bundle can be rejected because its binary or catalogue
contains matching text.

evidence: an archive with empty pax headers and ordinary content
`42 LIBARCHIVE.xattr.name=value` triggers `verify_archive_metadata`.

resolved when: the checker inspects actual archive headers, accepts matching
ordinary content, and still rejects genuine xattr metadata. verify with real
archives through the release checker; preserve the other artifact checks.
