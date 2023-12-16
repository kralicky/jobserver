package cgroupsv1

import (
	"errors"

	"github.com/kralicky/jobserver/pkg/cgroups"
	"github.com/kralicky/jobserver/pkg/jobs"
)

func init() {
	jobs.RegisterRuntime(cgroups.Version1, func() (jobs.Runtime, error) {
		return nil, errors.New("cgroupsv1 not supported")
	})
}
