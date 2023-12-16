package jobs

import (
	"context"
	"errors"

	jobv1 "github.com/kralicky/jobserver/pkg/apis/job/v1"
)

// Runtime represents a specific runtime environment that can be used to
// execute jobs.
type Runtime interface {
	// Creates a new job from the given spec and context, starts it, and returns
	// a Process interface that can be used to control the underlying process
	// described by the command in the job spec.
	//
	// The context controls the lifetime of the job; if the context is canceled
	// before the job completes, it will be terminated. Canceling the context
	// after the job completes has no effect.
	//
	// If the context is canceled with its cause matching ErrStoppedByUser, the
	// `stopped` field of the returned JobStatus will be set to true. This can
	// be used to distinguish between a job that was terminated from a signal
	// initiated by the user and a job that was terminated from a signal sent
	// by the system or by external means.
	//
	// This method will return an error if the spec is invalid.
	Execute(ctx context.Context, spec *jobv1.JobSpec) (Process, error)
}

// RuntimeId is a string name that can also be used as a key into LookupRuntime.
type RuntimeId string

// ErrStoppedByUser is a sentinel error that is used to indicate that a job
// was terminated by a signal initiated by the user.
//
// Correct usage of this error is such that context.Cause(ctx) matches this
// error according to [errors.Is], where 'ctx' was the same context passed to
// [exec.CommandContext] for the process.
var ErrStoppedByUser = errors.New("job stopped by user")

var allRuntimes = make(map[RuntimeId]RuntimeBuilder)

type RuntimeBuilder func() (Runtime, error)

func RegisterRuntime(id RuntimeId, builder RuntimeBuilder) {
	allRuntimes[id] = builder
}

func LookupRuntime(id RuntimeId) (RuntimeBuilder, bool) {
	builder, ok := allRuntimes[id]
	return builder, ok
}
