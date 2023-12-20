package commands

import (
	context "context"

	jobv1 "github.com/kralicky/jobserver/pkg/apis/job/v1"
)

type (
	jobClientContextKeyType struct{}
)

var jobClientContextKey jobClientContextKeyType

func ContextWithJobClient(ctx context.Context, client jobv1.JobClient) context.Context {
	return context.WithValue(ctx, jobClientContextKey, client)
}

func jobClientFromContext(ctx context.Context) (jobv1.JobClient, bool) {
	client, ok := ctx.Value(jobClientContextKey).(jobv1.JobClient)
	return client, ok
}
