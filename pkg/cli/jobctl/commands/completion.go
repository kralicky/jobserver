package commands

import (
	"slices"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/emptypb"
)

func completeJobIds(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if err := cmd.Root().PersistentPreRunE(cmd, args); err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	client, ok := jobClientFromContext(cmd.Context())
	if !ok {
		return nil, cobra.ShellCompDirectiveError
	}
	resp, err := client.List(cmd.Context(), &emptypb.Empty{})
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	ids := make([]string, 0, len(resp.GetItems()))
	for _, job := range resp.GetItems() {
		jobId := job.GetId()
		if slices.Contains(args, jobId) {
			continue
		}
		ids = append(ids, jobId)
	}
	slices.Sort(ids)
	return ids, cobra.ShellCompDirectiveNoFileComp
}
