package cgroupsv2

import (
	"context"
	"errors"
	"log/slog"
	"os/exec"
	"sync"
	"syscall"

	jobv1 "github.com/kralicky/jobserver/pkg/apis/job/v1"
	"github.com/kralicky/jobserver/pkg/jobs"
	"github.com/kralicky/jobserver/pkg/util"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type v2Process struct {
	id         string
	cmd        *exec.Cmd
	cmdContext context.Context
	streamBuf  *util.StreamBuffer
	done       chan struct{}

	statusMu sync.Mutex
	status   *jobv1.JobStatus
}

func (j *v2Process) ID() string {
	return j.id
}

func (j *v2Process) start() {
	lg := slog.With(
		"command", j.cmd.Path,
		"driver", "cgroupsv2",
	)

	j.statusMu.Lock()
	defer j.statusMu.Unlock()

	if err := j.cmd.Start(); err != nil {
		lg.Error("failed to start command")
		j.streamBuf.Close()
		close(j.done)
		j.status.State = jobv1.State_FAILED
		j.status.Message = err.Error()
		return
	}
	j.status.StartTime = timestamppb.Now()
	j.status.State = jobv1.State_RUNNING
	j.status.Message = jobv1.State_RUNNING.String()
	j.status.Pid = int32(j.cmd.Process.Pid)
	lg.Info("command started")

	go func() {
		defer j.streamBuf.Close()
		defer close(j.done)
		j.cmd.Wait()
		endTime := timestamppb.Now()

		j.statusMu.Lock()
		defer j.statusMu.Unlock()

		ws := j.cmd.ProcessState.Sys().(syscall.WaitStatus)
		term := &jobv1.TerminationStatus{
			Stopped: errors.Is(context.Cause(j.cmdContext), jobs.ErrStoppedByUser),
			Time:    endTime,
		}
		if ws.Exited() {
			term.ExitCode = int32(ws.ExitStatus())
		}
		if ws.Signaled() {
			term.Signal = int32(ws.Signal())
		}

		j.status.State = jobv1.State_TERMINATED
		j.status.Message = j.cmd.ProcessState.String()
		j.status.Terminated = term

		lg.With(
			"exitCode", ws.ExitStatus(),
			"signal", ws.Signal(),
			"stopped", term.Stopped,
			"duration", endTime.AsTime().Sub(j.status.GetStartTime().AsTime()),
		).Info("command terminated")
	}()
}

func (j *v2Process) Output(ctx context.Context) <-chan []byte {
	return j.streamBuf.NewStream(ctx)
}

func (j *v2Process) Status() *jobv1.JobStatus {
	j.statusMu.Lock()
	defer j.statusMu.Unlock()
	return proto.Clone(j.status).(*jobv1.JobStatus)
}

func (j *v2Process) Done() <-chan struct{} {
	return j.done
}

var _ jobs.Process = (*v2Process)(nil)
