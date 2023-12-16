package cgroupsv2

import (
	"bytes"
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
	info = bytes.TrimSpace(info)
	return strings.Fields(string(info)), nil
}

func enableController(file, name string) (retErr error) {
	f, err := os.OpenFile(file, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(f, "+%s\n", name)
	return errors.Join(err, f.Close())
}

const (
	CFSPeriod   = 100000
	CFSMinQuota = 1000
)

var availableMilliCpus = int64(runtime.NumCPU() * 1000)

func mcpusToCfsQuota(milliCores int64) int64 {
	return max(CFSMinQuota, int64(min(float64(milliCores)/float64(availableMilliCpus), 1.0)*CFSPeriod))
}

func writeCpuMaxQuota(path string, quota int64) error {
	f, err := os.OpenFile(filepath.Join(path, "cpu.max"), os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(f, "%d %d\n", quota, CFSPeriod)
	return errors.Join(err, f.Close())
}

func writeMemoryHigh(path string, high int64) error {
	f, err := os.OpenFile(filepath.Join(path, "memory.high"), os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(f, "%d\n", high)
	return errors.Join(err, f.Close())
}

func writeMemoryMax(path string, max int64) error {
	f, err := os.OpenFile(filepath.Join(path, "memory.max"), os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(f, "%d\n", max)
	return errors.Join(err, f.Close())
}

func writeIoMax(path, deviceId string, ioLimits *jobv1.IOLimits) error {
	f, err := os.OpenFile(filepath.Join(path, "io.max"), os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	var args []string
	if ioLimits.ReadBps != nil {
		args = append(args, fmt.Sprintf("rbps=%d", ioLimits.GetReadBps()))
	}
	if ioLimits.WriteBps != nil {
		args = append(args, fmt.Sprintf("wbps=%d", ioLimits.GetWriteBps()))
	}
	if ioLimits.ReadIops != nil {
		args = append(args, fmt.Sprintf("riops=%d", ioLimits.GetReadIops()))
	}
	if ioLimits.WriteIops != nil {
		args = append(args, fmt.Sprintf("wiops=%d", ioLimits.GetWriteIops()))
	}
	_, err = fmt.Fprintf(f, "%s %s\n", deviceId, strings.Join(args, " "))
	return errors.Join(err, f.Close())
}

func killCgroup(path string) error {
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
	_, err = syscall.InotifyAddWatch(fd, filepath.Join(path, "cgroup.events"), syscall.IN_MODIFY|syscall.IN_ONESHOT)
	if err != nil {
		return err
	}

	// write '1' to cgroup.kill
	f, err := os.OpenFile(filepath.Join(path, "cgroup.kill"), os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	if _, err := f.Write([]byte{'1'}); err != nil {
		return err
	}
	f.Close()

	// wait for cgroup.events to be modified
	start := time.Now()
	slog.Debug("killed cgroup; waiting for event signal")
	var buf [syscall.SizeofInotifyEvent]byte
	for {
		_, err := syscall.Read(fd, buf[:])
		if err != nil {
			if err == syscall.EINTR {
				continue
			}
			return err
		}
		break
	}
	slog.Debug("cgroup killed successfully", "took", time.Since(start))
	return nil
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
