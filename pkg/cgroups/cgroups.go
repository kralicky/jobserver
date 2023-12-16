package cgroups

import (
	"fmt"
	"syscall"

	"github.com/kralicky/jobserver/pkg/jobs"
)

const (
	Version1 = "cgroupsv1"
	Version2 = "cgroupsv2"

	// defined at https://github.com/torvalds/linux/blob/master/include/uapi/linux/magic.h#L69-L70
	Version1Magic = 0x27e0eb
	Version2Magic = 0x63677270
)

func DetectRuntime() (jobs.RuntimeId, error) {
	for {
		var stat syscall.Statfs_t
		err := syscall.Statfs("/sys/fs/cgroup", &stat)
		if err != nil {
			if err == syscall.EINTR {
				continue
			}
			return "", fmt.Errorf("failed to statfs /sys/fs/cgroup: %w", err)
		}
		switch stat.Type {
		case Version1Magic:
			return Version1, nil
		case Version2Magic:
			return Version2, nil
		default:
			return "", fmt.Errorf("unknown filesystem type at /sys/fs/cgroup: %x", stat.Type)
		}
	}
}
