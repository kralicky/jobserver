package cgroupsv2

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/google/uuid"
	jobv1 "github.com/kralicky/jobserver/pkg/apis/job/v1"
	"github.com/kralicky/jobserver/pkg/cgroups"
	"github.com/kralicky/jobserver/pkg/jobs"
	"github.com/kralicky/jobserver/pkg/util"
)

const gracePeriod = 10 * time.Second

type v2Runtime struct {
	mgr *cgroupManager
}

func newRuntime() (jobs.Runtime, error) {
	mgr, err := newCgroupManager()
	if err != nil {
		return nil, fmt.Errorf("failed to setup jobserver cgroup: %w", err)
	}

	return &v2Runtime{
		mgr: mgr,
	}, nil
}

// Execute implements jobs.Runtime.
func (l *v2Runtime) Execute(ctx context.Context, spec *jobv1.JobSpec) (jobs.Process, error) {
	cmdSpec := spec.GetCommand()

	// generate a uuid for the process, but encode it in the raw hex format.
	// this makes it slightly easier to use from the command line, since many
	// terminals treat '-' as a word separator, making it difficult to copy
	// the whole string using a double-click.
	u := uuid.New()
	id := hex.EncodeToString(u[:])

	cmd := exec.CommandContext(ctx, cmdSpec.GetCommand(), cmdSpec.GetArgs()...)
	cmd.Env = append(os.Environ(), cmdSpec.GetEnv()...)

	streamBuf := util.NewStreamBuffer()
	done := make(chan struct{})

	cmd.Stdout = streamBuf
	cmd.Stderr = streamBuf
	cmd.Stdin = nil
	cmd.Cancel = func() error {
		lg := slog.With("id", id)
		lg.Debug("context canceled; attempting graceful shutdown")
		streamBuf.Close() // NB: leaving this open will cause cmd.Wait to hang
		start := time.Now()
		go func() {
			timeout := time.NewTimer(gracePeriod)
			defer timeout.Stop()
			select {
			case <-timeout.C:
				lg.Warn("process did did not exit within grace period, sending SIGKILL")
				if err := cmd.Process.Signal(syscall.SIGKILL); err != nil {
					lg.Error("failed to send SIGKILL", "error", err)
				}
			case <-done:
				lg.Debug("process exited within grace period", "took", time.Since(start))
			}
		}()
		return cmd.Process.Signal(syscall.SIGTERM)
	}

	job := &v2Process{
		id:         id,
		streamBuf:  streamBuf,
		cmd:        cmd,
		cmdContext: ctx,
		done:       done,
		status: &jobv1.JobStatus{
			State:   jobv1.State_PENDING,
			Message: jobv1.State_PENDING.String(),
			Spec:    spec,
		},
	}

	if err := l.configureCgroup(job, id, spec.GetLimits()); err != nil {
		job.status.State = jobv1.State_FAILED
		job.status.Message = err.Error()
		return nil, err
	}

	job.start()

	return job, nil
}

// configureCgroup configures cgroup limits for the job.
func (l *v2Runtime) configureCgroup(job *v2Process, id string, limits *jobv1.ResourceLimits) error {
	path, err := l.mgr.CreateCgroupWithLimits(id, limits)
	if err != nil {
		return fmt.Errorf("failed to create cgroup for job %s: %w", id, err)
	}
	var cf int
	for {
		cf, err = syscall.Open(path, syscall.O_RDONLY|syscall.O_CLOEXEC, 0)
		if err != nil {
			if err == syscall.EINTR {
				continue
			}
			return fmt.Errorf("failed to open cgroup %s: %w", path, err)
		}
		break
	}
	job.cmd.SysProcAttr = &syscall.SysProcAttr{
		UseCgroupFD: true,
		CgroupFD:    cf,
	}
	go func() {
		<-job.Done()
		if err := syscall.Close(cf); err != nil {
			slog.Error("failed to close cgroup file descriptor", "path", path, "error", err)
		}
		if err := killCgroup(path); err != nil {
			slog.Error("failed to kill cgroup", "path", path, "error", err)
		}
		if err := os.Remove(path); err != nil {
			slog.Error("failed to remove cgroup", "path", path, "error", err)
		} else {
			slog.Info("removed cgroup", "path", path)
		}
	}()
	return nil
}

var _ jobs.Runtime = (*v2Runtime)(nil)

const Magic = 0x63677270

func init() {
	jobs.RegisterRuntime(cgroups.NewFilesystemRuntimeID(Magic), func() (jobs.Runtime, error) {
		return newRuntime()
	})
}
