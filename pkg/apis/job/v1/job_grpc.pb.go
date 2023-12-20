// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             (unknown)
// source: github.com/kralicky/jobserver/pkg/apis/job/v1/job.proto

package jobv1

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	Job_Start_FullMethodName  = "/job.v1.Job/Start"
	Job_Stop_FullMethodName   = "/job.v1.Job/Stop"
	Job_Status_FullMethodName = "/job.v1.Job/Status"
	Job_List_FullMethodName   = "/job.v1.Job/List"
	Job_Output_FullMethodName = "/job.v1.Job/Output"
)

// JobClient is the client API for Job service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type JobClient interface {
	// Starts a new job, and returns its id if it was started successfully.
	//
	// The job will be run asynchronously; this method does not wait for it
	// to complete.
	//
	// The job's process will be run in its own cgroup which will exist for the
	// lifetime of the process. Optionally, all resource limits set in the
	// 'limits' field will be applied to the cgroup before it is started.
	Start(ctx context.Context, in *JobSpec, opts ...grpc.CallOption) (*JobId, error)
	// Stops a running job. Once stopped, this method will wait until the job has
	// completed before returning.
	//
	// This will first attempt to stop the job's process using SIGTERM, but if the
	// process does not exit within a short grace period, it will be forcefully
	// killed with SIGKILL.
	//
	// A job must be in the Running state to be stopped. If the job is in any
	// other state, this returns a FailedPrecondition error.
	Stop(ctx context.Context, in *JobId, opts ...grpc.CallOption) (*emptypb.Empty, error)
	// Returns the status of an existing job.
	//
	// If the job is completed, detailed termination status will be present in
	// the response. Jobs stopped by the user with the Stop() method will
	// additionally have the 'stopped' field set to true.
	//
	// In the event the job failed to start, the 'message' field will contain a
	// human-readable error message. Otherwise, it will contain a description of
	// the current state (if running), or an explanation for why the job was
	// terminated (if terminated).
	Status(ctx context.Context, in *JobId, opts ...grpc.CallOption) (*JobStatus, error)
	// Returns a list of all job IDs that are currently known to the server.
	//
	// All jobs, regardless of state, are included. No guarantees are made about
	// the order of ids in the list.
	List(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*JobIdList, error)
	// Streams the combined stdout and stderr output of a running or completed job.
	//
	// The output always starts from the beginning of process execution, and will
	// be written to the stream in real-time until the job completes, or until the
	// stream is cancelled by the client.
	//
	// Jobs that were stopped by the user with the Stop() method will have output
	// up to the time they were stopped.
	//
	// If the job is already completed, the full output of the job will be
	// written to the stream, after which the stream will be closed.
	Output(ctx context.Context, in *JobId, opts ...grpc.CallOption) (Job_OutputClient, error)
}

type jobClient struct {
	cc grpc.ClientConnInterface
}

func NewJobClient(cc grpc.ClientConnInterface) JobClient {
	return &jobClient{cc}
}

func (c *jobClient) Start(ctx context.Context, in *JobSpec, opts ...grpc.CallOption) (*JobId, error) {
	out := new(JobId)
	err := c.cc.Invoke(ctx, Job_Start_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *jobClient) Stop(ctx context.Context, in *JobId, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, Job_Stop_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *jobClient) Status(ctx context.Context, in *JobId, opts ...grpc.CallOption) (*JobStatus, error) {
	out := new(JobStatus)
	err := c.cc.Invoke(ctx, Job_Status_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *jobClient) List(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*JobIdList, error) {
	out := new(JobIdList)
	err := c.cc.Invoke(ctx, Job_List_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *jobClient) Output(ctx context.Context, in *JobId, opts ...grpc.CallOption) (Job_OutputClient, error) {
	stream, err := c.cc.NewStream(ctx, &Job_ServiceDesc.Streams[0], Job_Output_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &jobOutputClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Job_OutputClient interface {
	Recv() (*ProcessOutput, error)
	grpc.ClientStream
}

type jobOutputClient struct {
	grpc.ClientStream
}

func (x *jobOutputClient) Recv() (*ProcessOutput, error) {
	m := new(ProcessOutput)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// JobServer is the server API for Job service.
// All implementations must embed UnimplementedJobServer
// for forward compatibility
type JobServer interface {
	// Starts a new job, and returns its id if it was started successfully.
	//
	// The job will be run asynchronously; this method does not wait for it
	// to complete.
	//
	// The job's process will be run in its own cgroup which will exist for the
	// lifetime of the process. Optionally, all resource limits set in the
	// 'limits' field will be applied to the cgroup before it is started.
	Start(context.Context, *JobSpec) (*JobId, error)
	// Stops a running job. Once stopped, this method will wait until the job has
	// completed before returning.
	//
	// This will first attempt to stop the job's process using SIGTERM, but if the
	// process does not exit within a short grace period, it will be forcefully
	// killed with SIGKILL.
	//
	// A job must be in the Running state to be stopped. If the job is in any
	// other state, this returns a FailedPrecondition error.
	Stop(context.Context, *JobId) (*emptypb.Empty, error)
	// Returns the status of an existing job.
	//
	// If the job is completed, detailed termination status will be present in
	// the response. Jobs stopped by the user with the Stop() method will
	// additionally have the 'stopped' field set to true.
	//
	// In the event the job failed to start, the 'message' field will contain a
	// human-readable error message. Otherwise, it will contain a description of
	// the current state (if running), or an explanation for why the job was
	// terminated (if terminated).
	Status(context.Context, *JobId) (*JobStatus, error)
	// Returns a list of all job IDs that are currently known to the server.
	//
	// All jobs, regardless of state, are included. No guarantees are made about
	// the order of ids in the list.
	List(context.Context, *emptypb.Empty) (*JobIdList, error)
	// Streams the combined stdout and stderr output of a running or completed job.
	//
	// The output always starts from the beginning of process execution, and will
	// be written to the stream in real-time until the job completes, or until the
	// stream is cancelled by the client.
	//
	// Jobs that were stopped by the user with the Stop() method will have output
	// up to the time they were stopped.
	//
	// If the job is already completed, the full output of the job will be
	// written to the stream, after which the stream will be closed.
	Output(*JobId, Job_OutputServer) error
	mustEmbedUnimplementedJobServer()
}

// UnimplementedJobServer must be embedded to have forward compatible implementations.
type UnimplementedJobServer struct {
}

func (UnimplementedJobServer) Start(context.Context, *JobSpec) (*JobId, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Start not implemented")
}
func (UnimplementedJobServer) Stop(context.Context, *JobId) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Stop not implemented")
}
func (UnimplementedJobServer) Status(context.Context, *JobId) (*JobStatus, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Status not implemented")
}
func (UnimplementedJobServer) List(context.Context, *emptypb.Empty) (*JobIdList, error) {
	return nil, status.Errorf(codes.Unimplemented, "method List not implemented")
}
func (UnimplementedJobServer) Output(*JobId, Job_OutputServer) error {
	return status.Errorf(codes.Unimplemented, "method Output not implemented")
}
func (UnimplementedJobServer) mustEmbedUnimplementedJobServer() {}

// UnsafeJobServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to JobServer will
// result in compilation errors.
type UnsafeJobServer interface {
	mustEmbedUnimplementedJobServer()
}

func RegisterJobServer(s grpc.ServiceRegistrar, srv JobServer) {
	s.RegisterService(&Job_ServiceDesc, srv)
}

func _Job_Start_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(JobSpec)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(JobServer).Start(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Job_Start_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(JobServer).Start(ctx, req.(*JobSpec))
	}
	return interceptor(ctx, in, info, handler)
}

func _Job_Stop_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(JobId)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(JobServer).Stop(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Job_Stop_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(JobServer).Stop(ctx, req.(*JobId))
	}
	return interceptor(ctx, in, info, handler)
}

func _Job_Status_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(JobId)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(JobServer).Status(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Job_Status_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(JobServer).Status(ctx, req.(*JobId))
	}
	return interceptor(ctx, in, info, handler)
}

func _Job_List_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(JobServer).List(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Job_List_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(JobServer).List(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _Job_Output_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(JobId)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(JobServer).Output(m, &jobOutputServer{stream})
}

type Job_OutputServer interface {
	Send(*ProcessOutput) error
	grpc.ServerStream
}

type jobOutputServer struct {
	grpc.ServerStream
}

func (x *jobOutputServer) Send(m *ProcessOutput) error {
	return x.ServerStream.SendMsg(m)
}

// Job_ServiceDesc is the grpc.ServiceDesc for Job service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Job_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "job.v1.Job",
	HandlerType: (*JobServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Start",
			Handler:    _Job_Start_Handler,
		},
		{
			MethodName: "Stop",
			Handler:    _Job_Stop_Handler,
		},
		{
			MethodName: "Status",
			Handler:    _Job_Status_Handler,
		},
		{
			MethodName: "List",
			Handler:    _Job_List_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Output",
			Handler:       _Job_Output_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "github.com/kralicky/jobserver/pkg/apis/job/v1/job.proto",
}
