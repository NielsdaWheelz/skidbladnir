//go:build darwin && cgo

package process

/*
#cgo LDFLAGS: -lproc
#include <errno.h>
#include <libproc.h>
#include <sys/sysctl.h>

// Both helpers return 0 on success and an errno-style cause on failure so Go
// can classify absent (ESRCH) and protected (EPERM/EACCES) processes.
static int skid_proc_info(int pid, struct proc_bsdinfo *info, char *path, int path_len, int *foreground) {
  errno = 0;
  if (proc_pidinfo(pid, PROC_PIDTBSDINFO, 0, info, sizeof(*info)) != sizeof(*info)) return errno != 0 ? errno : EINVAL;
  errno = 0;
  if (proc_pidpath(pid, path, path_len) <= 0) return errno != 0 ? errno : EINVAL;
  int mib[4] = { CTL_KERN, KERN_PROC, KERN_PROC_PID, pid };
  struct kinfo_proc kp; size_t size = sizeof(kp);
  errno = 0;
  if (sysctl(mib, 4, &kp, &size, NULL, 0) != 0) return errno != 0 ? errno : EINVAL;
  if (size != sizeof(kp)) return ESRCH; // the kernel returns an empty record for an absent pid
  *foreground = kp.kp_eproc.e_tpgid;
  return 0;
}

static int skid_foreground_process_group(int pid, int *foreground) {
  int mib[4] = { CTL_KERN, KERN_PROC, KERN_PROC_PID, pid };
  struct kinfo_proc kp; size_t size = sizeof(kp);
  errno = 0;
  if (sysctl(mib, 4, &kp, &size, NULL, 0) != 0) return errno != 0 ? errno : EINVAL;
  if (size != sizeof(kp)) return ESRCH; // the kernel returns an empty record for an absent pid
  *foreground = kp.kp_eproc.e_tpgid;
  return 0;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"strconv"
	"syscall"
	"unsafe"
)

func observeOnce(pid PID) (Observation, error) {
	var info C.struct_proc_bsdinfo
	path := make([]byte, C.PROC_PIDPATHINFO_MAXSIZE)
	var foreground C.int
	if code := C.skid_proc_info(C.int(pid), &info, (*C.char)(unsafe.Pointer(&path[0])), C.int(len(path)), &foreground); code != 0 {
		return Observation{}, classifyDarwinError(syscall.Errno(code), "observe Darwin process identity")
	}
	argv, err := darwinArgv(pid)
	if err != nil {
		return Observation{}, err
	}
	start := uint64(info.pbi_start_tvsec)*1_000_000 + uint64(info.pbi_start_tvusec)
	if start == 0 {
		return Observation{}, errors.New("Darwin process start identity is zero")
	}
	return Observation{PID: pid, ParentPID: PID(info.pbi_ppid), ProcessGroup: PID(info.pbi_pgid), ForegroundProcessGroup: PID(foreground), Executable: C.GoString((*C.char)(unsafe.Pointer(&path[0]))), Argv: argv, StartIdentity: StartIdentity(strconv.FormatUint(start, 10))}, nil
}

func classifyDarwinError(cause syscall.Errno, action string) error {
	switch cause {
	case syscall.ESRCH:
		return ErrProcessAbsent
	case syscall.EPERM, syscall.EACCES:
		return ErrProcessNotPermitted
	default:
		return fmt.Errorf("%s: %w", action, cause)
	}
}

func foregroundProcessGroup(panePID PID) (PID, error) {
	var foreground C.int
	if code := C.skid_foreground_process_group(C.int(panePID), &foreground); code != 0 {
		return 0, classifyDarwinError(syscall.Errno(code), "observe Darwin pane process")
	}
	if foreground <= 0 {
		return 0, errors.New("pane has no foreground process group")
	}
	return PID(foreground), nil
}

func darwinArgv(pid PID) ([]string, error) {
	mib := []C.int{C.CTL_KERN, C.KERN_ARGMAX}
	var size C.size_t
	var argmax C.int
	size = C.size_t(unsafe.Sizeof(argmax))
	if C.sysctl(&mib[0], 2, unsafe.Pointer(&argmax), &size, nil, 0) != 0 || argmax <= 0 {
		return nil, errors.New("read Darwin argument limit")
	}
	buffer := make([]byte, int(argmax))
	mib = []C.int{C.CTL_KERN, C.KERN_PROCARGS2, C.int(pid)}
	size = C.size_t(len(buffer))
	if result, err := C.sysctl(&mib[0], 3, unsafe.Pointer(&buffer[0]), &size, nil, 0); result != 0 {
		return nil, classifyDarwinArgvError(err)
	}
	buffer = buffer[:int(size)]
	if len(buffer) < int(unsafe.Sizeof(C.int(0))) {
		return nil, errors.New("Darwin process arguments are incomplete")
	}
	argc := *(*C.int)(unsafe.Pointer(&buffer[0]))
	offset := int(unsafe.Sizeof(argc))
	for offset < len(buffer) && buffer[offset] != 0 {
		offset++
	}
	for offset < len(buffer) && buffer[offset] == 0 {
		offset++
	}
	argv := make([]string, 0, int(argc))
	for len(argv) < int(argc) && offset < len(buffer) {
		start := offset
		for offset < len(buffer) && buffer[offset] != 0 {
			offset++
		}
		if offset == len(buffer) {
			return nil, errors.New("Darwin process argument is unterminated")
		}
		argv = append(argv, string(buffer[start:offset]))
		offset++
	}
	if len(argv) != int(argc) {
		return nil, errors.New("Darwin process argument count is incomplete")
	}
	return argv, nil
}

func classifyDarwinArgvError(err error) error {
	var cause syscall.Errno
	if !errors.As(err, &cause) {
		return fmt.Errorf("read Darwin process arguments: %w", err)
	}
	// KERN_PROCARGS2 reports EINVAL, not EPERM, for a process outside the
	// caller's observation domain; skid_proc_info has already distinguished an
	// absent pid (ESRCH) before argv is read, so EINVAL here is the privilege
	// boundary.
	if cause == syscall.EINVAL {
		return ErrProcessNotPermitted
	}
	return classifyDarwinError(cause, "read Darwin process arguments")
}
