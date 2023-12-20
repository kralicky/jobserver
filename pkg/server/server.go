package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net"
	"os"
	"runtime"
	"sync"
	"time"

	jobv1 "github.com/kralicky/jobserver/pkg/apis/job/v1"
	"github.com/kralicky/jobserver/pkg/auth"
	"github.com/kralicky/jobserver/pkg/jobs"
	"github.com/kralicky/jobserver/pkg/rbac"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Options struct {
	ListenAddress   string
	CaCertFile      string
	CertFile        string
	KeyFile         string
	AuthMiddlewares []auth.Middleware
}

type jobInfo struct {
	jobs.Process
	owner  auth.AuthenticatedUser
	cancel context.CancelCauseFunc
}

type Server struct {
	Options
	jobv1.UnsafeJobServer
	jobs    sync.Map // map[string]jobInfo
	runtime jobs.Runtime
}

func NewServer(runtime jobs.Runtime, options Options) *Server {
	return &Server{
		Options: options,
		runtime: runtime,
	}
}

type userJobId struct {
	*jobv1.JobId
	user auth.AuthenticatedUser
}

func (i userJobId) AssignedUser() auth.AuthenticatedUser {
	return i.user
}

// ListJobs implements v1.JobServer.
func (s *Server) List(ctx context.Context, _ *emptypb.Empty) (*jobv1.JobIdList, error) {
	var jobIds []userJobId
	s.jobs.Range(func(k, v any) bool {
		jobIds = append(jobIds, userJobId{
			JobId: &jobv1.JobId{Id: k.(string)},
			user:  v.(jobInfo).owner,
		})
		return true
	})

	var err error
	jobIds, err = rbac.FilterByScope(ctx, jobIds)
	if err != nil {
		return nil, err
	}

	var items []*jobv1.JobId
	for _, id := range jobIds {
		items = append(items, id.JobId)
	}
	return &jobv1.JobIdList{
		Items: items,
	}, nil
}

func (s *Server) lookupScoped(ctx context.Context, id *jobv1.JobId) (jobInfo, error) {
	var user auth.AuthenticatedUser
	// if the job doesn't exist, don't short circuit
	job, ok := s.jobs.Load(id.Id)
	if ok {
		user = job.(jobInfo).owner
	}
	if err := rbac.VerifyScopeForUser(ctx, user); err != nil {
		return jobInfo{}, err
	}
	if !ok {
		return jobInfo{}, status.Errorf(codes.NotFound, "job %s not found", id.Id)
	}
	return job.(jobInfo), nil
}

// GetStatus implements v1.JobServer.
func (s *Server) Status(ctx context.Context, id *jobv1.JobId) (*jobv1.JobStatus, error) {
	job, err := s.lookupScoped(ctx, id)
	if err != nil {
		return nil, err
	}
	return job.Status(), nil
}

// Start implements v1.JobServer.
func (s *Server) Start(ctx context.Context, in *jobv1.JobSpec) (*jobv1.JobId, error) {
	user := auth.AuthenticatedUserFromContext(ctx)
	jobCtx, cancel := context.WithCancelCause(context.TODO())
	proc, err := s.runtime.Execute(jobCtx, in)
	if err != nil {
		cancel(err)
		slog.With("error", err).Error("failed to start job")
		return nil, err
	}

	id := proc.ID()

	s.jobs.Store(id, jobInfo{
		Process: proc,
		owner:   user,
		cancel:  cancel,
	})

	return &jobv1.JobId{Id: id}, nil
}

// Stop implements v1.JobServer.
func (s *Server) Stop(ctx context.Context, id *jobv1.JobId) (*emptypb.Empty, error) {
	jobValue, ok := s.jobs.Load(id.Id)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "job %s not found", id.Id)
	}
	job := jobValue.(jobInfo)
	switch job.Status().GetState() {
	case jobv1.State_RUNNING:
	default:
		return nil, status.Errorf(codes.FailedPrecondition, "job %s is not running", id.Id)
	}

	job.cancel(jobs.ErrStoppedByUser)
	select {
	case <-job.Done():
		return &emptypb.Empty{}, nil
	case <-ctx.Done():
		return nil, status.FromContextError(ctx.Err()).Err()
	}
}

const maxChunkSize = 512 * 1024 // 512 KiB

// Output implements v1.JobServer.
func (s *Server) Output(id *jobv1.JobId, stream jobv1.Job_OutputServer) error {
	jobValue, ok := s.jobs.Load(id.Id)
	if !ok {
		return status.Errorf(codes.NotFound, "job %s not found", id.Id)
	}
	job := jobValue.(jobInfo)

	for buf := range job.Output(stream.Context()) {
		for len(buf) > 0 {
			chunk := buf
			if len(chunk) > maxChunkSize {
				chunk = chunk[:maxChunkSize]
			}
			buf = buf[len(chunk):]
			if err := stream.Send(&jobv1.ProcessOutput{Output: chunk}); err != nil {
				return err
			}
		}
	}

	return nil
}

var _ jobv1.JobServer = (*Server)(nil)

func (s *Server) ListenAndServe(ctx context.Context) error {
	cacertData, err := os.ReadFile(s.CaCertFile)
	if err != nil {
		return fmt.Errorf("failed to read CA certificate: %w", err)
	}
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(cacertData)

	cert, err := tls.LoadX509KeyPair(s.CertFile, s.KeyFile)
	if err != nil {
		return fmt.Errorf("failed to load server certificate: %w", err)
	}
	tlsConfig := &tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS13,
	}
	server := grpc.NewServer(
		grpc.Creds(credentials.NewTLS(tlsConfig)),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             15 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    15 * time.Second,
			Timeout: 5 * time.Second,
		}),
		grpc.NumStreamWorkers(uint32(runtime.NumCPU())),
		grpc.ChainStreamInterceptor(auth.StreamServerInterceptor(s.AuthMiddlewares)),
		grpc.ChainUnaryInterceptor(auth.UnaryServerInterceptor(s.AuthMiddlewares)),
	)
	jobv1.RegisterJobServer(server, s)

	listener, err := net.Listen("tcp", s.ListenAddress)
	if err != nil {
		return err
	}
	defer listener.Close()

	slog.With(
		"address", s.ListenAddress,
	).Info("job server starting")

	errC := make(chan error)
	go func() {
		err := server.Serve(listener)
		if err != nil {
			slog.With(
				"error", err,
			).Error("job server exited with error")
		} else {
			slog.Info("job server stopped")
		}
		errC <- err
	}()

	select {
	case <-ctx.Done():
		server.Stop()
		return (<-errC)
	case err := <-errC:
		return err
	}
}
