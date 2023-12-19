package jobv1

import (
	context "context"
)

type (
	jobClientContextKeyType struct{}
)

var jobClientContextKey jobClientContextKeyType

func ContextWithClient(ctx context.Context, client JobClient) context.Context {
	return context.WithValue(ctx, jobClientContextKey, client)
}

func ClientFromContext(ctx context.Context) (JobClient, bool) {
	client, ok := ctx.Value(jobClientContextKey).(JobClient)
	return client, ok
}
