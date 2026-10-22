// Package logging formats the serve process's content-free event lines and owns the bounded file
// they go to on the mac.
package logging

import (
	"io"
	"log/slog"
	"os"
)

// New returns the process's one event logger. Each Info call writes one JSON object on one line
// in a single Write: "time", "event.name" (the message), then the attributes in call order; no
// level. darwin's serve.log has no journal to stamp it, so the line carries its own time. Lines
// are content-free: ids, closed labels, codes, counts
// and durations only; never names, titles, cwd, paths, objectives, terminal bytes, raw errors,
// argv or credentials. Pass durations as int64 milliseconds, enums as strings and slices non-nil:
// slog writes a Duration in nanoseconds, an error or Errno as its text and a nil slice as null.
// Create one per process: the handler's lock is what keeps lines whole. Write errors are dropped.
func New(out io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(out, &slog.HandlerOptions{ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
		switch attr.Key {
		case slog.LevelKey:
			return slog.Attr{}
		case slog.MessageKey:
			attr.Key = "event.name"
		}
		return attr
	}}))
}

const limit = 8 << 20

// BoundedFile appends lines to path. Before a line would carry a non-empty file past 8 MiB it
// renames path to path+".1", replacing it, and starts a new file, so at most two files exist and
// no line is split. It opens lazily (O_APPEND, 0600, size from stat, so a restart continues the
// bound) and reopens after any failure, which costs that line: a missing or read-only directory
// heals on the first line after it is fixed; a file deleted under the writer keeps taking lines
// into the unlinked inode until the next rotation point, where the failed rename makes the next
// line start a new file. Writes are unbuffered: a process crash loses nothing written, and a crash
// mid-rotation leaves path+".1" whole and path absent or intact; a host crash may lose the
// unsynced tail, and a full disk can leave one partial line. Not safe for concurrent use: give it
// to one New, whose handler serializes writes. Two processes on one path (a restart overlapping
// its predecessor) keep lines whole through O_APPEND, but each counts only its own bytes, so a
// file can approach twice the bound and one rotation can replace the other's path+".1".
func BoundedFile(path string) io.Writer {
	return &boundedFile{path: path}
}

type boundedFile struct {
	path string
	file *os.File
	size int64
}

func (sink *boundedFile) Write(line []byte) (int, error) {
	if sink.file == nil {
		if err := sink.open(); err != nil {
			return 0, err
		}
	}
	if sink.size > 0 && sink.size+int64(len(line)) > limit {
		_ = sink.file.Close() // every byte written is already in the file
		sink.file = nil
		if err := os.Rename(sink.path, sink.path+".1"); err != nil {
			return 0, err
		}
		if err := sink.open(); err != nil {
			return 0, err
		}
	}
	written, err := sink.file.Write(line)
	sink.size += int64(written)
	if err != nil {
		_ = sink.file.Close()
		sink.file = nil // the next line reopens and re-reads the size
	}
	return written, err
}

func (sink *boundedFile) open() error {
	file, err := os.OpenFile(sink.path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return err
	}
	sink.file, sink.size = file, info.Size()
	return nil
}
