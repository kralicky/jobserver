package commands

import (
	"slices"

	jobv1 "github.com/kralicky/jobserver/pkg/apis/job/v1"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/emptypb"
)

func completeJobIds(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if err := cmd.Root().PersistentPreRunE(cmd, args); err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	client, ok := jobv1.ClientFromContext(cmd.Context())
	if !ok {
		return nil, cobra.ShellCompDirectiveError
	}
	resp, err := client.List(cmd.Context(), &emptypb.Empty{})
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	var ids []string
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
