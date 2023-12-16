package jobs

import (
	"context"

	jobv1 "github.com/kralicky/jobserver/pkg/apis/job/v1"
)

// Process represents a read-only view of the underlying process of a job that
// was started by a Runtime, and can be used to query the status of the process,
// as well as to stream its output.
type Process interface {
	// Returns the unique ID of the process.
	ID() string
	// Streams the combined stdout and stderr of the process to the returned
	// channel. Data will be written to the channel in real time until either the
	// job terminates or the provided context is canceled, after which the channel
	// will be closed.
	// If the job is already terminated when this method is called, the full
	// output will be written to the channel, and the channel will be closed
	// immediately.
	// Each call to this method will return a new independent channel that will
	// receive a copy of the output. Safe to call concurrently from multiple
	// goroutines.
	Output(ctx context.Context) <-chan []byte
	// Returns the current status of the job. This method is safe to call
	// concurrently from multiple goroutines.
	Status() *jobv1.JobStatus
	// Returns a channel that will be closed when the job terminates.
	// Successive calls to Done() will return the same channel.
	Done() <-chan struct{}
}
