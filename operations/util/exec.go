package util

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	pb "github.com/eparis/admin-rpc/api"
)

type streamWriter struct {
	stream pb.RemoteCommand_SendCommandServer
}

func (sw streamWriter) Write(p []byte) (int, error) {
	cr := &pb.CommandReply{
		Output: p,
	}
	if err := sw.stream.Send(cr); err != nil {
		return 0, err
	}
	return len(p), nil
}

var (
	selfNamespace = Namespaces{}
	initNamespace = Namespaces{
		Mount: 1,
		Uts:   1,
		IPC:   1,
		Net:   1,
		Pid:   1,
		// User:   1,
		// Cgroup: 1,
		Root: 1,
		Cwd:  1,
	}
)

type Namespaces struct {
	Mount int // /proc/pid/ns/mnt
	Uts   int // /proc/pid/ns/uts
	IPC   int // /proc/pid/ns/ipc
	Net   int // /proc/pid/ns/net
	Pid   int // /proc/pid/ns/pid
	// The RHEL7 version of nsenter does not support User or Cgroup
	// User   int // /proc/pid/ns/user
	// Cgroup int // /proc/pid/ns/cgroup
	Root int // /proc/pid/root
	Cwd  int // /proc/pid/cwd
}

func (n Namespaces) args() []string {
	args := []string{}
	if n.Mount != 0 {
		args = append(args, fmt.Sprintf("--mount=/proc/%d/ns/mnt", n.Mount))
	} else {
		args = append(args, "--mount=/proc/self/ns/mnt")
	}
	if n.Uts != 0 {
		args = append(args, fmt.Sprintf("--uts=/proc/%d/ns/uts", n.Uts))
	} else {
		args = append(args, "--uts=/proc/self/ns/uts")
	}
	if n.IPC != 0 {
		args = append(args, fmt.Sprintf("--ipc=/proc/%d/ns/ipc", n.IPC))
	} else {
		args = append(args, "--ipc=/proc/self/ns/ipc")
	}
	if n.Net != 0 {
		args = append(args, fmt.Sprintf("--net=/proc/%d/ns/net", n.Net))
	} else {
		args = append(args, "--net=/proc/self/ns/net")
	}
	if n.Pid != 0 {
		args = append(args, fmt.Sprintf("--pid=/proc/%d/ns/pid", n.Pid))
	} else {
		args = append(args, "--pid=/proc/self/ns/pid")
	}
	/*
		if n.User != 0 {
			args = append(args, fmt.Sprintf("--user=/proc/%d/ns/user", n.User))
		} else {
			args = append(args, "--user=/proc/self/ns/user")
		}
		if n.Cgroup != 0 {
			args = append(args, fmt.Sprintf("--cgroup=/proc/%d/ns/cgroup", n.Cgroup))
		} else {
			args = append(args, "--cgroup=/proc/self/ns/cgroup")
		}
	*/
	if n.Root != 0 {
		args = append(args, fmt.Sprintf("--root=/proc/%d/root", n.Root))
	} else {
		args = append(args, "--root=/proc/self/root")
	}
	if n.Cwd != 0 {
		args = append(args, fmt.Sprintf("--wd=/proc/%d/cwd", n.Cwd))
	} else {
		args = append(args, "--wd=/proc/self/cwd")
	}
	args = append(args, "--")
	return args
}

func ExecuteCmdNamespace(cmdName string, args []string, ns Namespaces, stream pb.RemoteCommand_SendCommandServer) error {
	outPipe, pw, err := os.Pipe()
	if err != nil {
		return err
	}

	nsenterArgs := append(ns.args(), cmdName)
	nsenterArgs = append(nsenterArgs, args...)
	cmd := exec.Command("nsenter", nsenterArgs...)
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		return err
	}

	finished := make(chan bool)

	// When the process ends, close the pipe. This will cause the io.Copy() to
	// hit EOF and return.
	go func() {
		cmd.Wait()
		pw.Close()
	}()

	// If the client closes the stream mark that we are finished so we may
	// stop the exec early if needed.
	go func() {
		select {
		case <-stream.Context().Done():
			finished <- true
		}
	}()

	// If the io.Copy() returned that means we either hit an error or outPipe
	// return EOF. In either case, we've done all we can do, so indicate we
	// are finished and should return.
	go func() {
		defer func() {
			finished <- true
			outPipe.Close()
		}()
		sw := streamWriter{
			stream: stream,
		}
		for {
			l, err := io.Copy(sw, outPipe)
			if err != nil || l == 0 {
				return
			}
		}
	}()

	select {
	case <-finished:
		// If the process is still running after we are finished
		// we should kill it. After the call to Wait() cmd.ProcessState
		// should be non-nil.
		if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
			cmd.Process.Kill()
		}
	}

	return nil
}

func ExecuteCmdSelfNS(cmdName string, args []string, stream pb.RemoteCommand_SendCommandServer) error {
	return ExecuteCmdNamespace(cmdName, args, selfNamespace, stream)
}

func ExecuteCmdInitNS(cmdName string, args []string, stream pb.RemoteCommand_SendCommandServer) error {
	return ExecuteCmdNamespace(cmdName, args, initNamespace, stream)
}
