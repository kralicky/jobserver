package cgroups

import (
	"fmt"
	"syscall"

	"github.com/kralicky/jobserver/pkg/jobs"
)

func NewFilesystemRuntimeID(magic int64) jobs.RuntimeID {
	return jobs.RuntimeID(fmt.Sprintf("fs://%x", magic))
}

func DetectFilesystemRuntime() (jobs.RuntimeID, error) {
	for {
		var stat syscall.Statfs_t
		err := syscall.Statfs("/sys/fs/cgroup", &stat)
		if err != nil {
			if err == syscall.EINTR {
				continue
			}
			return jobs.RuntimeID(""), fmt.Errorf("failed to statfs /sys/fs/cgroup: %w", err)
		}
		return NewFilesystemRuntimeID(stat.Type), nil
	}
}
