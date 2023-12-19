package cgroupsv1

import (
	"errors"

	"github.com/kralicky/jobserver/pkg/cgroups"
	"github.com/kralicky/jobserver/pkg/jobs"
)

const (
	// defined at https://github.com/torvalds/linux/blob/master/include/uapi/linux/magic.h#L69-L70
	Magic = 0x27e0eb
)

func init() {
	jobs.RegisterRuntime(cgroups.NewFilesystemRuntimeID(Magic), func() (jobs.Runtime, error) {
		return nil, errors.New("cgroupsv1 not supported")
	})
}
