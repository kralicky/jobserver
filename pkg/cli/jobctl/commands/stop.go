package commands

import (
	"fmt"

	jobv1 "github.com/kralicky/jobserver/pkg/apis/job/v1"
	"github.com/spf13/cobra"
)

func BuildJobStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "stop <job-id>",
		GroupID: GroupIdClientCommands,
		Short:   "Stop a running job.",
		Long: `
Stops a running job, then waits for the job to be terminated.

This will first attempt to stop the job's process using SIGTERM, but if the
process does not exit within a short grace period, it will be forcefully
killed with SIGKILL.
`[1:],
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeJobIds,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, ok := jobClientFromContext(cmd.Context())
			if !ok {
				cmd.PrintErrln("failed to get client from context")
				return nil
			}
			_, err := client.Stop(cmd.Context(), &jobv1.JobId{Id: args[0]})
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), args[0])
			return nil
		},
	}
	return cmd
}
