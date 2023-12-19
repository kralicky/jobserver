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

// RuntimeID is an opaque string id that can also be used as a key into LookupRuntime.
type RuntimeID string

// ErrStoppedByUser is a sentinel error that is used to indicate that a job
// was terminated by a signal initiated by the user.
//
// Correct usage of this error is such that context.Cause(ctx) matches this
// error according to [errors.Is], where 'ctx' was the same context passed to
// [exec.CommandContext] for the process.
var ErrStoppedByUser = errors.New("job stopped by user")

var allRuntimes = make(map[RuntimeID]RuntimeBuilder)

type RuntimeBuilder func() (Runtime, error)

// RegisterRuntime registers a new runtime with the given id. This must only
// be called from an init() function.
//
// To link in a new runtime, use a blank import (ideally in package main).
//
// For example:
//
//	package main
//	import (
//		_ "github.com/kralicky/jobserver/pkg/cgroups/cgroupsv2"
//	)
func RegisterRuntime(id RuntimeID, builder RuntimeBuilder) {
	allRuntimes[id] = builder
}

// LookupRuntime returns a previously registered runtime with the given id.
// The runtime must have been linked in to the currntly running binary.
func LookupRuntime(id RuntimeID) (RuntimeBuilder, bool) {
	builder, ok := allRuntimes[id]
	return builder, ok
}
