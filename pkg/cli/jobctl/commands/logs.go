package commands

import (
	"errors"
	"io"

	jobv1 "github.com/kralicky/jobserver/pkg/apis/job/v1"
	"github.com/spf13/cobra"
)

func BuildJobLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "logs <job-id>",
		GroupID: GroupIdClientCommands,
		Short:   "Stream the output of an existing job.",
		Long: `
Streams the output of the combined stdout and stderr of an existing job.

If the job is still running, this will continue to stream the output in real
time until either the job terminates, or the command is interrupted with Ctrl-C.

If the job has already terminated, its full output will be written and the
command will exit.
`[1:],
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeJobIds,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, ok := jobv1.ClientFromContext(cmd.Context())
			if !ok {
				cmd.PrintErrln("failed to get client from context")
				return nil
			}
			stream, err := client.Output(cmd.Context(), &jobv1.JobId{Id: args[0]})
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
		},
	}
	return cmd
}
