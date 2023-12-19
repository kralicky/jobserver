package commands

import (
	"fmt"

	jobv1 "github.com/kralicky/jobserver/pkg/apis/job/v1"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
)

func BuildJobStatusCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:     "status <job-id>",
		GroupID: GroupIdClientCommands,
		Short:   "Show the status of an existing job.",
		Long: `
Shows the status of an existing job, including current state, pid, original
spec, start and end time, and exit status (if applicable).
`[1:],
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeJobIds,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, ok := jobv1.ClientFromContext(cmd.Context())
			if !ok {
				cmd.PrintErrln("failed to get client from context")
				return nil
			}
			status, err := client.Status(cmd.Context(), &jobv1.JobId{Id: args[0]})
			if err != nil {
				return err
			}
			switch output {
			case "json":
				fmt.Fprintln(cmd.OutOrStdout(), protojson.Format(status))
			case "text":
				fmt.Fprintln(cmd.OutOrStdout(), prototext.Format(status))
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "json", "Output format (json|text)")
	cmd.RegisterFlagCompletionFunc("output", cobra.FixedCompletions([]string{"json", "text"}, cobra.ShellCompDirectiveNoFileComp))
	return cmd
}
