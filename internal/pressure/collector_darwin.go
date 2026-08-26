//go:build darwin && cgo

package pressure

/*
#include <mach/mach.h>
#include <math.h>
#include <stdlib.h>
#include <sys/mount.h>
#include <sys/sysctl.h>

struct skid_pressure_sample {
  uint64_t cpu_total;
  uint64_t cpu_idle;
  double load;
  int logical_cpus;
  int memory_pressure;
  double swap_total;
  double swap_used;
  uint64_t disk_blocks;
  uint64_t disk_available;
  unsigned int valid;
};

static struct skid_pressure_sample skid_collect_pressure(void) {
  struct skid_pressure_sample out = {0};
  host_cpu_load_info_data_t cpu; mach_msg_type_number_t count = HOST_CPU_LOAD_INFO_COUNT;
  mach_port_t host = mach_host_self();
  kern_return_t cpu_result = KERN_FAILURE;
  if (MACH_PORT_VALID(host)) {
    cpu_result = host_statistics(host, HOST_CPU_LOAD_INFO, (host_info_t)&cpu, &count);
    mach_port_deallocate(mach_task_self(), host);
  }
  if (cpu_result == KERN_SUCCESS) {
    out.cpu_idle = cpu.cpu_ticks[CPU_STATE_IDLE];
    for (int i = 0; i < CPU_STATE_MAX; i++) out.cpu_total += cpu.cpu_ticks[i];
    out.valid |= 1;
  }
  if (getloadavg(&out.load, 1) == 1 && out.load >= 0) out.valid |= 2;
  size_t size = sizeof(out.logical_cpus);
  if (sysctlbyname("hw.logicalcpu", &out.logical_cpus, &size, NULL, 0) == 0 && out.logical_cpus > 0) out.valid |= 4;
  size = sizeof(out.memory_pressure);
  if (sysctlbyname("kern.memorystatus_vm_pressure_level", &out.memory_pressure, &size, NULL, 0) == 0) out.valid |= 8;
  struct xsw_usage swap; size = sizeof(swap);
  if (sysctlbyname("vm.swapusage", &swap, &size, NULL, 0) == 0) { out.swap_total = swap.xsu_total; out.swap_used = swap.xsu_used; out.valid |= 16; }
  struct statfs disk;
  if (statfs("/", &disk) == 0 && disk.f_blocks > 0) { out.disk_blocks = disk.f_blocks; out.disk_available = disk.f_bavail; out.valid |= 32; }
  return out;
}

static int skid_host_port_send_right_references(mach_port_urefs_t *references) {
  mach_port_t host = mach_host_self();
  if (!MACH_PORT_VALID(host)) return -1;
  kern_return_t result = mach_port_get_refs(mach_task_self(), host, MACH_PORT_RIGHT_SEND, references);
  mach_port_deallocate(mach_task_self(), host);
  return result == KERN_SUCCESS ? 0 : -1;
}
*/
import "C"

type cpuCounters struct {
	total uint64
	idle  uint64
}
type collector struct {
	previousCPU    cpuCounters
	hasPreviousCPU bool
}

func newCollector() collector { return collector{} }
func currentPolicy() policy   { return darwinPolicy() }

func hostPortSendRightReferences() (uint32, bool) {
	var references C.mach_port_urefs_t
	if C.skid_host_port_send_right_references(&references) != 0 {
		return 0, false
	}
	return uint32(references), true
}

func (collector *collector) collect() rawSample {
	raw := C.skid_collect_pressure()
	sample := rawSample{}
	if raw.valid&1 != 0 {
		current := cpuCounters{total: uint64(raw.cpu_total), idle: uint64(raw.cpu_idle)}
		if collector.hasPreviousCPU {
			sample.cpuPercent = cpuUsage(collector.previousCPU, current)
		}
		collector.previousCPU, collector.hasPreviousCPU = current, true
	}
	if raw.valid&6 == 6 {
		sample.loadNormalized = knownMetric(float64(raw.load) / float64(raw.logical_cpus))
	}
	if raw.valid&8 != 0 {
		switch int(raw.memory_pressure) {
		case 1:
			sample.memoryPressure = knownMemoryPressure(MemoryPressureNormal)
		case 2:
			sample.memoryPressure = knownMemoryPressure(MemoryPressureWarning)
		case 4:
			sample.memoryPressure = knownMemoryPressure(MemoryPressureCritical)
		}
	}
	if raw.valid&16 != 0 {
		if raw.swap_total == 0 {
			sample.swapUsedPercent = knownMetric(0)
		} else if raw.swap_used <= raw.swap_total {
			sample.swapUsedPercent = knownMetric(float64(raw.swap_used) * 100 / float64(raw.swap_total))
		}
	}
	if raw.valid&32 != 0 && raw.disk_available <= raw.disk_blocks {
		sample.diskAvailablePercent = knownMetric(float64(raw.disk_available) * 100 / float64(raw.disk_blocks))
	}
	return sample
}

func cpuUsage(previous, current cpuCounters) metric {
	if current.total <= previous.total || current.idle < previous.idle {
		return metric{}
	}
	total, idle := current.total-previous.total, current.idle-previous.idle
	if idle > total {
		return metric{}
	}
	return knownMetric(float64(total-idle) * 100 / float64(total))
}
