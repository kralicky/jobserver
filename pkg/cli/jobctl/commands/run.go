package commands

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	jobv1 "github.com/kralicky/jobserver/pkg/apis/job/v1"
	"github.com/spf13/cobra"
)

func BuildJobRunCmd() *cobra.Command {
	var env []string
	var workdir string
	var cpus string
	var memory string
	var memorySoftLimit string
	var deviceReadBps []string
	var deviceWriteBps []string
	var deviceReadIops []string
	var deviceWriteIops []string
	var follow bool

	cmd := &cobra.Command{
		Use:     "run [flags] -- <command> [args...]",
		Aliases: []string{"start"},
		GroupID: GroupIdClientCommands,
		Short:   "Run a new job.",
		Long: fmt.Sprintf(`
Starts a new job, and prints its ID if it was started successfully.

The job will be run in the background; this command does not wait for the job
to complete.

To check the status of the job, use the command '%[1]s status <id>'.
To stream the output of the job, use the command '%[1]s logs <id>'.
`[1:], os.Args[0]),
		Example: fmt.Sprintf(`
  Commands that don't require flag args can be passed as-is:
    $ %[1]s run kubectl logs pod/example

  Commands that do require flag args must be delimited with '--':
    $ %[1]s run -- kubectl logs --follow pod/example

  Starting a job with resource limits will apply all limits to the job's cgroup.
    $ %[1]s run \
       --cpus=100m \
       --memory=1Gi \
       --device-read-bps=/dev/sda=200 \
       --device-write-bps=/dev/sda=50,/dev/sdb=100 \
       -- go build -o bin/jobserver ./cmd/jobserver
`[1:], os.Args[0]),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, ok := jobv1.ClientFromContext(cmd.Context())
			if !ok {
				cmd.PrintErrln("failed to get client from context")
				return nil
			}
			cmdSpec := &jobv1.CommandSpec{
				Command: args[0],
				Env:     env,
			}
			if len(args) > 1 {
				cmdSpec.Args = args[1:]
			}
			limits := &jobv1.ResourceLimits{}
			if cpus != "" {
				mcpus, err := parseCpuLimits(cpus)
				if err != nil {
					return fmt.Errorf("invalid value for cpu limit: %w", err)
				}
				limits.Cpu = &mcpus
			}
			if memorySoftLimit != "" || memory != "" {
				mem, err := parseMemoryLimits(memorySoftLimit, memory)
				if err != nil {
					return err
				}
				limits.Memory = mem
			}
			if len(deviceReadBps) > 0 || len(deviceWriteBps) > 0 || len(deviceReadIops) > 0 || len(deviceWriteIops) > 0 {
				devices, err := parseIoLimits(deviceReadBps, deviceWriteBps, deviceReadIops, deviceWriteIops)
				if err != nil {
					return err
				}
				limits.Io = devices
			}
			id, err := client.Start(cmd.Context(), &jobv1.JobSpec{
				Command: cmdSpec,
				Limits:  limits,
			})
			if err != nil {
				return err
			}
			if follow {
				stream, err := client.Output(cmd.Context(), id)
				if err != nil {
					return err
				}
				for {
					resp, err := stream.Recv()
					if err != nil {
						if errors.Is(err, io.EOF) {
							return nil
						}
						return err
					}
					cmd.OutOrStdout().Write(resp.GetOutput())
				}
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), id.Id)
			}
			return nil
		},
	}
	cmd.Flags().StringSliceVarP(&env, "env", "e", nil,
		"environment variables                       (ex: 'FOO=bar' or 'BAZ=qux')")
	cmd.Flags().StringVarP(&workdir, "workdir", "w", "", "working directory for the command (default is the server's working directory)")
	cmd.Flags().StringVarP(&cpus, "cpus", "c", "",
		"number of CPUs to allocate to the job       (ex: '4' '100m')")
	cmd.Flags().StringVarP(&memory, "memory", "m", "",
		"amount of memory to allocate to the job     (ex: '100Mi' or '256k' or '4G')")
	cmd.Flags().StringVar(&memorySoftLimit, "memory-soft-limit", "",
		"soft limit for memory usage                 (ex: '100Mi' or '256k' or '4G')")
	cmd.Flags().StringSliceVar(&deviceReadBps, "device-read-bps", nil,
		"device read bandwidth limits (id|path=bps)  (ex: '8:16=200' or '/dev/sda=200')")
	cmd.Flags().StringSliceVar(&deviceWriteBps, "device-write-bps", nil,
		"device write bandwidth limits (id|path=bps) (ex: '8:16=200' or '/dev/sda=200')")
	cmd.Flags().StringSliceVar(&deviceReadIops, "device-read-iops", nil,
		"device read IOPS limits (id|path=iops)      (ex: '8:16=200' or '/dev/sda=200')")
	cmd.Flags().StringSliceVar(&deviceWriteIops, "device-write-iops", nil,
		"device write IOPS limits (id|path=iops)     (ex: '8:16=200' or '/dev/sda=200')")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow the output of the job")
	return cmd
}

func parseCpuLimits(cpus string) (int64, error) {
	// valid formats:
	// - integer whole number (e.g. 2)
	// - integer ending in 'm' (e.g. 100m)
	// not supporting floating point numbers or any other suffixes

	if strings.HasSuffix(cpus, "m") {
		return strconv.ParseInt(cpus[:len(cpus)-1], 10, 64)
	}
	n, err := strconv.ParseInt(cpus, 10, 64)
	if err != nil {
		return 0, err
	}
	return n * 1000, nil
}

func parseMemoryLimits(softLimit, limit string) (*jobv1.MemoryLimits, error) {
	// valid formats:
	// - integer whole number with the suffix k, M, G, T
	// - integer whole number with the suffix Ki, Mi, Gi, Ti
	limits := &jobv1.MemoryLimits{}
	if softLimit != "" {
		softLimitBytes, err := parseMemoryLimit(softLimit)
		if err != nil {
			return nil, fmt.Errorf("invalid value for memory soft limit: %w", err)
		}
		limits.SoftLimit = &softLimitBytes
	}
	if limit != "" {
		limitBytes, err := parseMemoryLimit(limit)
		if err != nil {
			return nil, fmt.Errorf("invalid value for memory limit: %w", err)
		}
		limits.Limit = &limitBytes
	}
	return limits, nil
}

func parseMemoryLimit(limit string) (int64, error) {
	switch {
	case strings.HasSuffix(limit, "k"):
		k, err := strconv.ParseInt(limit[:len(limit)-1], 10, 64)
		if err != nil {
			return 0, err
		}
		return k * 1000, nil
	case strings.HasSuffix(limit, "M"):
		m, err := strconv.ParseInt(limit[:len(limit)-1], 10, 64)
		if err != nil {
			return 0, err
		}
		return m * 1000 * 1000, nil
	case strings.HasSuffix(limit, "G"):
		g, err := strconv.ParseInt(limit[:len(limit)-1], 10, 64)
		if err != nil {
			return 0, err
		}
		return g * 1000 * 1000 * 1000, nil
	case strings.HasSuffix(limit, "Ki"):
		ki, err := strconv.ParseInt(limit[:len(limit)-2], 10, 64)
		if err != nil {
			return 0, err
		}
		return ki * 1024, nil
	case strings.HasSuffix(limit, "Mi"):
		mi, err := strconv.ParseInt(limit[:len(limit)-2], 10, 64)
		if err != nil {
			return 0, err
		}
		return mi * 1024 * 1024, nil
	case strings.HasSuffix(limit, "Gi"):
		gi, err := strconv.ParseInt(limit[:len(limit)-2], 10, 64)
		if err != nil {
			return 0, err
		}
		return gi * 1024 * 1024 * 1024, nil
	default:
		return 0, fmt.Errorf("unknown memory limit format: %s (expecting k, M, G, Ki, Mi, Gi suffix)", limit)
	}
}

func parseIoLimits(readBps, writeBps, readIops, writeIops []string) ([]*jobv1.IODeviceLimits, error) {
	// valid formats:
	// - major:minor=limit
	// - path=limit

	devices := make(map[string]*jobv1.IOLimits)
	for _, bps := range readBps {
		dev, lim, err := parseIoLimit(bps)
		if err != nil {
			return nil, fmt.Errorf("invalid io limit format %q: %w", bps, err)
		}
		devices[dev] = &jobv1.IOLimits{ReadBps: &lim}
	}
	for _, bps := range writeBps {
		dev, lim, err := parseIoLimit(bps)
		if err != nil {
			return nil, fmt.Errorf("invalid io limit format %q: %w", bps, err)
		}
		if l, ok := devices[dev]; ok {
			l.WriteBps = &lim
		} else {
			devices[dev] = &jobv1.IOLimits{WriteBps: &lim}
		}
	}
	for _, iops := range readIops {
		dev, lim, err := parseIoLimit(iops)
		if err != nil {
			return nil, fmt.Errorf("invalid io limit format %q: %w", iops, err)
		}
		if l, ok := devices[dev]; ok {
			l.ReadIops = &lim
		} else {
			devices[dev] = &jobv1.IOLimits{ReadIops: &lim}
		}
	}
	for _, iops := range writeIops {
		dev, lim, err := parseIoLimit(iops)
		if err != nil {
			return nil, fmt.Errorf("invalid io limit format %q: %w", iops, err)
		}
		if l, ok := devices[dev]; ok {
			l.WriteIops = &lim
		} else {
			devices[dev] = &jobv1.IOLimits{WriteIops: &lim}
		}
	}

	var limits []*jobv1.IODeviceLimits
	for dev, l := range devices {
		limits = append(limits, &jobv1.IODeviceLimits{
			Device: dev,
			Limits: l,
		})
	}
	slices.SortFunc(limits, func(a, b *jobv1.IODeviceLimits) int {
		return cmp.Compare(a.Device, b.Device)
	})
	return limits, nil
}

func parseIoLimit(limit string) (string, int64, error) {
	dev, lim, ok := strings.Cut(limit, "=")
	if !ok {
		return "", 0, fmt.Errorf("expecting 'device=limit'")
	}
	limInt, err := strconv.ParseInt(lim, 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("invalid limit value: %w", err)
	}

	if !filepath.IsAbs(dev) {
		return "", 0, fmt.Errorf("expecting device id to be a path or 'major:minor' id")
	}
	return dev, limInt, nil
}
