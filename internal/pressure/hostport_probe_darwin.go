//go:build integration && darwin && cgo

// The Mach host-port reference probe exists only for the integration leak
// proof; cgo is not allowed in _test.go files, so it lives in this
// integration-tagged non-test file and stays out of production builds.

package pressure

/*
#include <mach/mach.h>

static int skid_host_port_send_right_references(mach_port_urefs_t *references) {
  mach_port_t host = mach_host_self();
  if (!MACH_PORT_VALID(host)) return -1;
  kern_return_t result = mach_port_get_refs(mach_task_self(), host, MACH_PORT_RIGHT_SEND, references);
  mach_port_deallocate(mach_task_self(), host);
  return result == KERN_SUCCESS ? 0 : -1;
}
*/
import "C"

func hostPortSendRightReferences() (uint32, bool) {
	var references C.mach_port_urefs_t
	if C.skid_host_port_send_right_references(&references) != 0 {
		return 0, false
	}
	return uint32(references), true
}
