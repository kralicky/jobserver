package cgroupsv2

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	jobv1 "github.com/kralicky/jobserver/pkg/apis/job/v1"
	"golang.org/x/sys/unix"
)

func listControllers(file string) ([]string, error) {
	info, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(info)), nil
}

func sysFsWrite(file string, str string) error {
	f, err := os.OpenFile(file, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(f, str)
	return errors.Join(err, f.Close())
}

func enableController(file, name string) (retErr error) {
	return sysFsWrite(file, fmt.Sprintf("+%s\n", name))
}

const (
	cfsPeriod   = 100000
	cfsMinQuota = 1000
)

// For the purposes of this project, we assume that the jobserver has the
// full resources of the machine available to it. This may not be true in
// a real-world scenario (for example, the jobserver may itself be running
// in a cgroup with limited resources).
var availableMilliCpus = int64(runtime.NumCPU() * 1000)

func mcpusToCfsQuota(milliCores int64) int64 {
	return max(cfsMinQuota, int64(min(float64(milliCores)/float64(availableMilliCpus), 1.0)*cfsPeriod))
}

func writeCpuMaxQuota(path string, quota int64) error {
	return sysFsWrite(filepath.Join(path, "cpu.max"), fmt.Sprintf("%d %d\n", quota, cfsPeriod))
}

func writeMemoryHigh(path string, high int64) error {
	return sysFsWrite(filepath.Join(path, "memory.high"), fmt.Sprintf("%d\n", high))
}

func writeMemoryMax(path string, max int64) error {
	return sysFsWrite(filepath.Join(path, "memory.max"), fmt.Sprintf("%d\n", max))
}

func writeIoMax(path, deviceId string, ioLimits *jobv1.IOLimits) error {
	builder := strings.Builder{}
	builder.WriteString(deviceId)
	sz := builder.Len()
	if ioLimits.ReadBps != nil {
		builder.WriteString(fmt.Sprintf(" rbps=%d", ioLimits.GetReadBps()))
	}
	if ioLimits.WriteBps != nil {
		builder.WriteString(fmt.Sprintf(" wbps=%d", ioLimits.GetWriteBps()))
	}
	if ioLimits.ReadIops != nil {
		builder.WriteString(fmt.Sprintf(" riops=%d", ioLimits.GetReadIops()))
	}
	if ioLimits.WriteIops != nil {
		builder.WriteString(fmt.Sprintf(" wiops=%d", ioLimits.GetWriteIops()))
	}
	if builder.Len() == sz { // no limits specified
		return nil
	}
	return sysFsWrite(filepath.Join(path, "io.max"), builder.String())
}

func writeCgroupKill(path string) error {
	return sysFsWrite(filepath.Join(path, "cgroup.kill"), "1")
}

func killCgroup(path string) error {
	slog.Debug("killing cgroup", "path", path)

	// start an inotify watcher on cgroup.events
	fd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC)
	if err != nil {
		return err
	}
	defer func() {
		for {
			if err := syscall.Close(fd); err != syscall.EINTR {
				return
			}
		}
	}()
	_, err = syscall.InotifyAddWatch(fd, filepath.Join(path, "cgroup.events"), syscall.IN_MODIFY)
	if err != nil {
		return err
	}

	// write '1' to cgroup.kill
	if err := writeCgroupKill(path); err != nil {
		return err
	}

	// wait for cgroup.events to be modified
	start := time.Now()
	var buf [syscall.SizeofInotifyEvent]byte
	for {
		if populated, err := isCgroupPopulated(path); err != nil {
			return err
		} else if !populated {
			break
		}
		slog.Debug("waiting for cgroup to become unpopulated", "path", path)
		_, err := syscall.Read(fd, buf[:])
		if err != nil {
			if errors.Is(err, syscall.EINTR) {
				continue
			}
			return err
		}
	}
	slog.Debug("cgroup killed successfully", "took", time.Since(start))
	return nil
}

func isCgroupPopulated(path string) (bool, error) {
	contents, err := os.ReadFile(filepath.Join(path, "cgroup.events"))
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(string(contents), "\n") {
		k, v, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		if k == "populated" {
			return v == "1", nil
		}
	}
	return false, nil
}

func lookupDeviceId(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", fmt.Errorf("failed to stat %s: no info available", path)
	}
	return fmt.Sprintf("%d:%d", unix.Major(stat.Rdev), unix.Minor(stat.Rdev)), nil
}
