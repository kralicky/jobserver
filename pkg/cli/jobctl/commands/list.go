package commands

import (
	"fmt"
	"slices"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	jobv1 "github.com/kralicky/jobserver/pkg/apis/job/v1"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/emptypb"
)

func BuildJobListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		GroupID: GroupIdClientCommands,
		Short:   "Show all existing jobs.",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, ok := jobv1.ClientFromContext(cmd.Context())
			if !ok {
				cmd.PrintErrln("failed to get client from context")
				return nil
			}
			list, err := client.List(cmd.Context(), &emptypb.Empty{})
			if err != nil {
				return err
			}
			tab := table.NewWriter()
			tab.AppendHeader(table.Row{"JOB ID", "COMMAND", "CREATED", "STATUS"})
			rows := make([]table.Row, 0, len(list.Items))
			for _, id := range list.Items {
				stat, err := client.Status(cmd.Context(), id)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "error obtaining status for %s: %s\n", id.Id, err.Error())
					continue
				}
				rows = append(rows, table.Row{
					id.GetId(),
					stat.GetSpec().GetCommand().GetCommand(),
					stat.GetStartTime().AsTime(),
					stat.GetMessage(),
				})
			}
			slices.SortFunc(rows, func(a, b table.Row) int {
				return a[2].(time.Time).Compare(b[2].(time.Time))
			})
			tab.AppendRows(rows)
			fmt.Fprintln(cmd.OutOrStdout(), tab.Render())
			return nil
		},
	}

	return cmd
}
