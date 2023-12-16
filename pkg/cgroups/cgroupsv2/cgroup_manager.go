package cgroupsv2

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"

	jobv1 "github.com/kralicky/jobserver/pkg/apis/job/v1"
)

const (
	hierarchyRootPath = "/sys/fs/cgroup"
	jobserverCgroup   = "kralicky-jobserver"
)

var (
	requiredControllers = []string{"cpu", "memory", "io"}
)

type cgroupManager struct {
	path string
}

func newCgroupManager() (*cgroupManager, error) {
	if ok, err := requiredControllersEnabled(filepath.Join(hierarchyRootPath, "cgroup.controllers")); !ok {
		return nil, err
	}
	if ok, err := requiredControllersEnabled(filepath.Join(hierarchyRootPath, "cgroup.subtree_control")); !ok {
		return nil, err
	}
	// create the jobserver cgroup if it doesn't exist
	jobserverCgroup := filepath.Join(hierarchyRootPath, jobserverCgroup)
	if _, err := os.Stat(jobserverCgroup); os.IsNotExist(err) {
		if err := os.Mkdir(jobserverCgroup, 0755); err != nil {
			return nil, fmt.Errorf("failed to create jobserver cgroup: %w", err)
		}
		slog.Info("created jobserver cgroup", "path", jobserverCgroup)
	}

	// ensure the required controllers are enabled
	if err := enableRequiredControllers(filepath.Join(jobserverCgroup, "cgroup.subtree_control")); err != nil {
		return nil, fmt.Errorf("failed to enable cgroup subtree controllers: %w", err)
	}
	slog.Info("initialized jobserver cgroup", "path", jobserverCgroup)
	return &cgroupManager{path: jobserverCgroup}, nil
}

func (m *cgroupManager) CreateCgroupWithLimits(id string, limits *jobv1.ResourceLimits) (string, error) {
	// create a new cgroup for the job
	path := filepath.Join(m.path, id)
	if err := os.Mkdir(path, 0755); err != nil {
		return "", fmt.Errorf("failed to create cgroup %s: %w", path, err)
	}
	slog.Info("created cgroup", "path", path, "job", id)

	if limits == nil {
		return path, nil
	}

	// set all present limits
	if limits.Cpu != nil {
		if err := writeCpuMaxQuota(path, mcpusToCfsQuota(limits.GetCpu())); err != nil {
			return "", fmt.Errorf("failed to set cpu.max: %w", err)
		}
	}
	if limits.Memory != nil {
		if limits.Memory.SoftLimit != nil {
			if err := writeMemoryHigh(path, limits.Memory.GetSoftLimit()); err != nil {
				return "", fmt.Errorf("failed to set memory.high: %w", err)
			}
		}
		if limits.Memory.Limit != nil {
			if err := writeMemoryMax(path, limits.Memory.GetLimit()); err != nil {
				return "", fmt.Errorf("failed to set memory.max: %w", err)
			}
		}
	}
	for _, dev := range limits.GetIo() {
		id, err := lookupDeviceId(dev.Device)
		if err != nil {
			return "", fmt.Errorf("failed to lookup device id for %s: %w", dev.Device, err)
		}
		if dev.Limits != nil {
			if err := writeIoMax(path, id, dev.Limits); err != nil {
				return "", fmt.Errorf("failed to set io.max for device %s: %w", id, err)
			}
		}
	}
	return path, nil
}

func requiredControllersEnabled(file string) (bool, error) {
	controllers, err := listControllers(file)
	if err != nil {
		return false, fmt.Errorf("failed to read cgroup controllers: %w", err)
	}
	for _, c := range requiredControllers {
		if !slices.Contains(controllers, c) {
			return false, fmt.Errorf("required cgroup controller %q is not enabled in %s", c, file)
		}
	}
	return true, nil
}

func enableRequiredControllers(file string) error {
	// enable the required subtree controllers
	enabledControllers, err := listControllers(file)
	if err != nil {
		return fmt.Errorf("failed to read controllers: %w", err)
	}
	for _, c := range requiredControllers {
		if !slices.Contains(enabledControllers, c) {
			slog.Info("enabling controller", "controller", c, "file", file)
			if err := enableController(file, c); err != nil {
				return fmt.Errorf("failed to enable controller %q: %w", c, err)
			}
		}
	}

	// verify that all required controllers are enabled
	if _, err := requiredControllersEnabled(file); err != nil {
		return fmt.Errorf("failed to enable required controllers: %w", err)
	}
	return nil
}
