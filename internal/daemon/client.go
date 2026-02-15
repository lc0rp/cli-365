package daemon

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"time"
)

func Call(socketPath string, req Request, timeout time.Duration) (Response, error) {
	if socketPath == "" {
		return Response{}, errors.New("socket path is required")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	conn, err := net.DialTimeout("unix", socketPath, timeout)
	if err != nil {
		return Response{}, err
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	enc := json.NewEncoder(conn)
	if err := enc.Encode(req); err != nil {
		return Response{}, err
	}

	var resp Response
	dec := json.NewDecoder(conn)
	if err := dec.Decode(&resp); err != nil {
		return Response{}, err
	}

	return resp, nil
}

func Ping(socketPath string, timeout time.Duration) error {
	resp, err := Call(socketPath, Request{
		RequestID: "ping",
		Command:   CommandPing,
	}, timeout)
	if err != nil {
		return err
	}
	if !resp.OK {
		return errors.New(resp.ErrorCode)
	}
	return nil
}

func Stop(socketPath string, timeout time.Duration) error {
	resp, err := Call(socketPath, Request{
		RequestID: "stop",
		Command:   CommandStop,
	}, timeout)
	if err != nil {
		return err
	}
	if !resp.OK {
		return errors.New(resp.ErrorCode)
	}
	return nil
}

func ReadStatus(path string) (Status, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Status{}, err
	}
	var status Status
	if err := json.Unmarshal(data, &status); err != nil {
		return Status{}, err
	}
	return status, nil
}

func StatusFromOptions(opts Options, timeout time.Duration) (Status, error) {
	opts = opts.withDefaults()

	status, err := ReadStatus(opts.StatusPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return Status{}, err
		}
		status = Status{
			Running:    false,
			SocketPath: opts.SocketPath,
			LockPath:   opts.LockPath,
		}
	}

	if err := Ping(opts.SocketPath, timeout); err == nil {
		status.Running = true
	} else {
		status.Running = false
		status.LastError = err.Error()
	}

	if status.SocketPath == "" {
		status.SocketPath = opts.SocketPath
	}
	if status.LockPath == "" {
		status.LockPath = opts.LockPath
	}

	return status, nil
}
