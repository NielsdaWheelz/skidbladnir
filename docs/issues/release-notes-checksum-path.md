# release-note checksum depends on directory spelling

problem: `render-release-notes` hashes the manifest by filename and keeps only
checksum output with two whitespace-separated fields. a release directory
containing spaces adds filename fields, so otherwise valid inputs are rejected.

impact: the standalone renderer rejects a valid quoted directory argument.
normal release staging currently uses a path without spaces. this was reproduced
with actual rendering and synthetic release metadata during tag-admission work.

resolved when: hash manifest bytes through stdin so the checksum tool's output
has no input filename to parse. retain canonical digest admission and error
propagation. preserve normal-path notes byte-for-byte and render the same
metadata from a directory containing spaces. no artifact, signing or publication
change; keep this separate from tag admission.
