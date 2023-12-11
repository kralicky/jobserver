---
authors: Joe Kralicky (joe@kralic.ky)
state: draft
---

# Job Server

## Required Approvers

- Engineering: `@smallinsky` && `@espadolini` && `@russjones` && `@romant`

## What

An implementation of a job worker service as described in the [Teleport Systems Challenge 1](https://github.com/gravitational/careers/blob/main/challenges/systems/challenge-1.md) document. This is the level 4 version of the challenge.

## Why

This project is part of the Teleport interview process. For more details, see the [Rationale section](https://github.com/gravitational/careers/blob/main/challenges/systems/challenge-1.md#rationale) in the challenge documentation.

## Scope

Due to the nature of this project, the following limitations will be made:

1. Only linux/amd64 architecture will be supported.
2. Only Cgroups v2 will be supported (however, sufficient abstraction will be put in place to allow implenting v1 support if desired).
3. The server must be run with root privileges.
4. Authorization will be limited in scope and will not impose restrictions on commands that can be run. The authorization model is limited to restricting access to specific API methods.
5. Test coverage will be limited to a few packages only.
6. Authentication will be limited to mTLS only, and will use pre-generated certificates.
7. Everything is stored in memory, and no persistent storage will be used. See [Security Considerations](#security-considerations) for more details.
8. Only a small subset of the possible ways to limit resources will be implemented:

   - cpu limits using cpu.max only, period hard coded to the default 100000, no weight/shares support, no cpuset support
   - memory limits using memory.high and memory.max only, no swap or hugepages support
   - device io limits using io.max only, no weight or latency support

## Disclaimer

This project is **not** intended for real-world use, and does not implement many of the the features that would be necessary to make it safe to use in a production environment.

## Details

The three main components of the job server are:

1. A server that provides a GRPC API to allow clients to start and stop jobs, list and query the status of existing jobs, and stream logs of running or completed jobs.
2. A set of generic and reusable library packages that are used to implement the server and API.
3. A user-friendly CLI client that allows users to interact with the job server.

### Proto Specification

The Job Server API is composed of a single service with five methods, defined as follows:

```protobuf
syntax = "proto3";

package job.v1;

import "google/protobuf/empty.proto";
import "google/protobuf/timestamp.proto";

option go_package = "github.com/kralicky/jobserver/pkg/apis/job/v1";

service Job {
  // Starts a new job, and returns its id if it was started successfully.
  //
  // The job will be run asynchronously; this method does not wait for it
  // to complete.
  //
  // If resource limits are provided, the job will be run in its own cgroup
  // with all the given limits applied. Any combination of the allowed limit
  // types may be provided; it is not necessary to set all of them.
  rpc Start(JobSpec) returns (JobId);

  // Stops a running job. Once stopped, this method will wait until the job has
  // completed before returning.
  //
  // This will first attempt to stop the job's process using SIGTERM, but if the
  // process does not exit within a short grace period, it will be forcefully
  // killed with SIGKILL.
  //
  // A job must be in the Running state to be stopped. If the job is in any
  // other state, this returns a FailedPrecondition error.
  rpc Stop(JobId) returns (google.protobuf.Empty);

  // Returns the status of an existing job.
  //
  // If the job is completed, the exit code and detailed wait status will be
  // present in the response. Jobs stopped by the user with the Stop() method
  // may not have an exit code, depending if the process was able to exit
  // cleanly within the grace period. Jobs that failed to start, or were
  // otherwise killed by the system, will not have an exit code, but may have
  // a detailed wait status indicating the reason for failure.
  rpc Status(JobId) returns (JobStatus);

  // Returns a list of all job IDs that are currently known to the server.
  //
  // All jobs, regardless of state, are included. No guarantees are made about
  // the order of ids in the list.
  rpc List(google.protobuf.Empty) returns (JobIdList);

  // Streams the combined stdout and stderr output of a running or completed job.
  // The output always starts from the beginning of process execution. Jobs that
  // were stopped by the user with the Stop() method will have output up to the
  // time they were stopped.
  //
  // If 'follow' is set to true, the output will continue to be be streamed in
  // real-time until the job completes. Otherwise, only the currently accumulated
  // output will be written, and the stream will be closed even if the job is
  // still running.
  //
  // If the job is already completed, the full output of the job will be
  // written to the stream, after which the stream will be closed.
  rpc Output(OutputRequest) returns (stream ProcessOutput);
}

// JobSpec describes a command to be run, along with optional resource limits
// that should be applied to the command's process.
message JobSpec {
  CommandSpec    command = 1;
  ResourceLimits limits  = 2;
}

message JobId {
  string id = 1;
}

message JobIdList {
  repeated JobId items = 1;
}

enum State {
  Unknown = 0;
  // The job is waiting to be started, and is not yet running.
  Pending = 1;
  // The job is currently running.
  Running = 2;
  // The job exited normally (with any exit code), or was terminated via signal.
  Completed = 3;
  // The job failed to start, or exited abnormally due to an I/O error.
  Failed = 5;
}

message JobStatus {
  // The job's logical state.
  State state = 1;
  // A human-readable message describing the job's state, or an error message
  // if the job failed.
  string message = 2;
  // The PID of the job's process.
  int32 pid = 3;
  // The exit code of the job's process.
  // Only present if the job is in the Completed state.
  optional int32 exitCode = 4;
  // A bitmask equivalent to syscall.WaitStatus, also described in wait(2).
  // Only present if the job is in the Completed or Failed state.
  optional uint32 waitStatus = 5;
  // The time at which the job was started.
  google.protobuf.Timestamp startTime = 6;
  // The time at which the job completed (or failed). If the job is still
  // running, this field will not be present.
  google.protobuf.Timestamp endTime = 7;
  // The job's original spec.
  JobSpec spec = 8;
}

message CommandSpec {
  // The command name to run. Required.
  string command = 1;
  // The command's arguments, not containing the command name itself.
  repeated string args = 2;
  // Optional additional environment variables to set for the command.
  // These will be merged with the job server's environment variables.
  repeated string env = 3;
  // The working directory for the command. If not specified, the job server's
  // working directory will be used.
  optional string cwd = 4;
  // Optional user id to run the command as.
  // If not specified, the job server's user id will be used.
  optional int32 uid = 5;
  // Optional group id to run the command as.
  // If not specified, the job server's group id will be used.
  optional int32 gid = 6;
  // Optional data to be written to the process's stdin pipe when it starts.
  // If not specified, the process's stdin will be set to /dev/null.
  optional bytes stdin = 7;
}

message ProcessOutput {
  // A chunk of output from the process's combined stdout and stderr streams.
  //
  // This is a raw byte stream, and may contain arbitrary binary data,
  // depending on the command being run. It is up to the client to interpret
  // the output correctly based on the command being run. No guarantees are
  // made about the size of each chunk, or the frequency at which they
  // are sent.
  bytes output = 1;
}

message ResourceLimits {
  // Process CPU limit defined in milli-cores (1000 = 1 core/vcpu)
  optional int64 cpu = 1;
  // Process memory limit defined in bytes
  optional MemoryLimits memory = 2;
  // Process IO limits for devices
  optional IODeviceLimits io = 3;
}

message MemoryLimits {
  // Memory soft limit defined in bytes. Processes exceeding the limit are
  // not OOM-killed, but may be throttled by the kernel.
  optional int64 softLimit = 1;
  // Memory limit defined in bytes. If exceeded, the process will be OOM-killed.
  optional int64 limit = 2;
}

message IODeviceLimits {
  // Limits for individual devices
  repeated DeviceLimit devices = 1;
}

message DeviceLimit {
  // Device id (major:minor)
  string deviceId = 1;
  // Limits for the device
  IOLimits limits = 2;
}

message IOLimits {
  // Limit for read operations in bytes per second
  optional int64 readBps = 2;
  // Limit for write operations in bytes per second
  optional int64 writeBps = 3;
  // Limit for read operations in IOPS
  optional int64 readIops = 4;
  // Limit for write operations in IOPS
  optional int64 writeIops = 5;
}

message OutputRequest {
  JobId id     = 1;
  bool  follow = 2;
}

```

Authorization uses a very simple RBAC system. The RBAC configuration is defined as follows:

```protobuf
syntax = "proto3";

package rbac.v1;

option go_package = "github.com/kralicky/jobserver/pkg/apis/rbac/v1";

// Describes a complete RBAC configuration.
message Config {
  // A list of available roles.
  repeated Roles roles = 2;
  // A list of available role bindings.
  repeated RoleBindings roleBindings = 3;
}

// Describes a role that allows access to methods within a service.
message Roles {
  // An arbitrary unique identifier for the role.
  string id = 1;
  // The service name to which the role applies. Should be qualified to
  // the full package name of the service, not including '/' separators.
  // For example, `service Foo` in `package bar.baz` should be "bar.baz.Foo".
  string service = 2;
  // A list of methods that the role allows access to. The method names must
  // not be qualified with the service name. All methods must exist in the
  // named service. For example, `rpc Bar` in `service Foo` should be "Bar".
  repeated string allowedMethods = 3;
}

// Describes a role binding, associating a single role with one or more subjects.
message RoleBindings {
  // An arbitrary unique identifier for the role binding.
  string id = 1;
  // An existing role id.
  string roleId = 2;
  // A list of subjects that the role applies to. A subject generally refers to
  // a named user or group, but the actual meaning is left as an implementation
  // detail for the server, as it largely depends on how its authentication
  // system is configured.
  repeated string subjects = 3;
}

```

The RBAC configuration will be simply loaded from a config file on server startup. An example config file might look like:

```yaml
roles:
  - id: adminRole
    service: job.v1.Job
    allowedMethods:
      - Start
      - Stop
      - Status
      - List
      - Output
roleBindings:
  - id: adminRoleBinding
    roleId: adminRole
    subjects:
      - admin
```

APIs and message types are versioned, initially starting at v1. This will allow future major versions to be introduced without breaking existing clients. Adding new fields to existing messages can be done without breaking compatibility, as long as existing field tags are not changed.

### UX

#### API

The methods of the Job service API are designed such that each method accomplishes a single task and requires the least amount of stateful knowledge as possible.

Other than `Start()` and `List()`, which are fully stateless, the only stateful parameter required for interacting with the various methods is a Job ID. Job IDs are completely opaque to the user, which reduces client complexity. Furthermore, the IDs are automatically given to the user upon starting a job, so the client has all the information it needs in order to perform any other operation for that job.

Example usage of the GRPC API would look like:

1. Starting a job:

```go
var client jobv1.JobClient
// ...
mcores := int64(1000) // 1 core
jobId, err := client.Start(cmd.Context(), &jobv1.JobSpec{
  Command: &jobv1.CommandSpec{
    Command: "kubectl",
    Args:    []string{"logs", "--follow", "pod/example"},
    Env:     []string{"KUBECONFIG=/path/to/kubeconfig"},
  },
  Limits: &jobv1.ResourceLimits{
    Cpu: &mcores,
  },
})
```

2. Streaming output:

```go
stream, err := client.Output(cmd.Context(), &jobv1.OutputRequest{
  Id:     jobId,
  Follow: true,
})
if err != nil {
  return err
}
for {
  resp, err := stream.Recv()
  if err != nil {
    return err
  }
  fmt.Fprint(os.Stdout, resp.GetOutput())
}
```

3. Stopping the job:

```go
_, err := client.Stop(ctx, jobId)
```

#### CLI

The CLI is the primary means of interacting with the Job Server. It will have a familiar interface to other CLI tools like `docker` or `kubectl`.
The following examples demonstrate how the CLI will be used:

```bash
# starting the job server
$ sudo jobserver serve --listen-address=127.0.0.1:9090 --rbac=rbac-config.yaml \
  --cacert=ca.crt --cert=tls.crt --key=tls.key

# to use the client, flags to configure cert and key paths, and the server address, are required.
# these are omitted in the remaining examples for brevity.
$ jobserver --address=127.0.0.1:9090 --cacert=ca.crt --cert=tls.crt --key=tls.key ...

# starting a job:
# for commands that don't require flag args, they can be passed as-is:
$ jobserver run kubectl logs pod/example
<job-id> # job id is printed

# for commands that do require flag args, the expected '--' delimiter can be used:
$ jobserver run -- kubectl logs --follow pod/example
<job-id>

# to start a job with resource limits:
$ jobserver run \
  --cpus=100m \ # 100 milli-cores; can also write e.g. '--cpus=2' for 2 cores (no float values)
  --memory=1Gi --memory-soft-limit=512Mi \ # the familiar suffixes can be used for memory limits
  --device-read-bps=8:16=200 \ # format is major:minor=value (bytes per second)
  --device-write-bps=8:16=200,8:0=50 \ # multiple devices can be specified by separating with commas
  --device-read-iops=... --device-write-iops=... \ # same format, but units are in iops
  -- go build -o bin/jobserver ./cmd/jobserver

# stopping a job:
$ jobserver stop <job-id>
<job-id>

# getting the status of a job:
$ jobserver status <job-id>
state: running
pid: 1234
start time: 2006-01-02T15:04:05Z07:00

# getting the output of a job:
$ jobserver logs [--follow] <job-id>
<output>

# getting a list of all job ids:
$ jobserver list
JOB ID      COMMAND       CREATED          STATUS
<id-1>      kubectl       2006-01-02...    Running
<id-2>      go            2006-01-02...    Completed
```

### Implementation Details

#### Streaming Output

Output streaming will be implemented as follows:

1. When a job is started, we will pipe the combined stdout and stderr of its process to a simple in-memory buffer.
2. When a client initiates a stream by calling `Output()`, we will write the current contents of the buffer to the stream in 4MB chunks. Then, if the client requested to follow the output, the server will keep the stream open, and any subsequent reads from the process's output will be duplicated to all clients that are following the stream, in addition to the server's own buffer.

#### Process Lifecycle

For jobs that do not specify resource limits, they will be run as a child process of the job server itself without any additional isolation. For jobs that do specify resource limits, they will be run in their own cgroup with all of the requested limits.

On startup, the server will do the following (simplifying, there will be additional abstractions here):

1. Ensure its euid is 0 so it can manage cgroups, and ensure that cgroups v2 is enabled.
2. Create a parent cgroup in `/user.slice/user-<uid>.slice/jobserver` if it doesn't exist. The specific path can be determined by first identifying its own cgroup from `/proc/self/cgroup` (which, if run as `sudo jobserver serve`, will be something like `/user.slice/user-<uid>.slice/<terminal emulator's cgroup>/vte-spawn-<some uuid>.scope`), then walking the tree up until it finds a cgroup matching `user-*.slice`.
3. Enable the `cpu`, `memory`, and `io` controllers in the `jobserver` cgroup's `cgroup.subtree_control` file, if necessary.

The lifecycle of a job's cgroup will be as follows:

1. When a job is created with resource limits, the server will create a new cgroup with the name `<job-id>` under the parent `jobserver` cgroup.
2. The server will write the requested resource limits to the new cgroup's `cpu.max`, `memory.max`, and `io.max` files (for whichever limits were specified).
3. The server will create a file descriptor to the new cgroup by opening its path (`/sys/fs/cgroup/user.slice/user-<uid>.slice/jobserver/<job-id>`).
4. The server will configure the `exec.Cmd`'s `SysProcAttr` with `CgroupFd` set to the file descriptor. When this is set, Go will call `clone3` with `CLONE_INTO_CGROUP` so that the new child process is started directly in the new cgroup.
5. When the process exits, the server will close the file descriptor and delete the job's cgroup.

Other notes:

- When clients are streaming logs for a running process, the server will close those streams once the process has exited and all logs have been written.
- The server will not attempt to modify the controllers of its parent cgroups, so if `user-<uid>.slice` doesn't have any of the required controlers enabled, it will simply bail out.
- The user will be able to override the default parent cgroup path with a flag, since there can be some variation in the cgroup hierarchy depending on the system.

### Security

#### Authentication and Authorization

Authentication is implemented using simple mTLS, with a subject name encoded in the client certificate's common name field.

TLS 1.3 will be enforced by the server. The Go implementation uses a fixed set of 3 very strong cipher suites - `chacha20-poly1305-sha256`, `aes-128-gcm-sha256`, and `aes-256-gcm-sha384` (no aes ccm modes) - and requires no additional configuration or tuning outside of setting the minimum TLS version to 1.3.

To authorize a user for a specific api method, the server will match the client's subject name against the rbac rules it loaded from the config file,
then check to see if that subject is named in a role binding for which the associated role allows access to the method the client is calling.

The client's certificate information can be obtained from from GRPC peer info inside a server interceptor. The peer info will store the client's credentials as a `credentials.TLSInfo` struct, which contains the client's verified chains. From there we can read the verified client cert's common name and use it for authorization.

#### Security Considerations

- **Denial of Service**: Since the simplified API defined here is not particularly optimized for high performance, and since scalability and HA were not design goals, the server is likely to be vulnerable to DoS attacks. Some improvements that can help address this, but are out of scope for the project include:

  - Server-side rate limiting (e.g. using a token bucket rate limiter from a library such as `golang.org/x/time/rate` in a GRPC interceptor)
  - Optimized RPC design to reduce the number of individual RPCs required to perform a given task. For example, any of the methods could be modified to operate on lists of job IDs, additional request parameters could be added to filter/limit/paginate results, etc. Additionally, methods like Status could be modified to use server-side streaming, so that clients would not need to repeatedly poll the server.
  - Scaling of job server instances using a key-value store and load balancer to distribute/shard requests.

- **Unconstrained Resource Usage**: All jobs, including their logs, will be stored in memory on the server. Persistent storage and other such server optimizations are out of scope for this project, but would be absolutely necessary for a production system. Simply running `jobserver run yes` would cause the server to run out of memory in seconds. Some possible improvements include:

  - Writing and streaming logs to disk instead of keeping them in memory
  - Storing logs compressed
  - Using a rolling buffer with a fixed size cap so that the server will only keep around the last N bytes of logs for each job
  - Expiring old jobs and/or logs after a certain amount of time, or only keeping track of the last N completed jobs
