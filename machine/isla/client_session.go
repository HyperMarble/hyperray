// A live isla-client: started once, asked for many instructions.
// A session that cannot start reports why; it never falls back to a process.
package isla

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

// ClientSession owns one isla-client process and the socket it answers on.
//
// The process reads and initialises the architecture once. Every instruction
// after that costs only its own execution.
type ClientSession struct {
	process    *exec.Cmd
	connection net.Conn
	listener   net.Listener
	directory  string
}

// StartClientSession launches isla-client and waits for it to connect back.
func StartClientSession(ctx context.Context, tool string, architecture string,
	configuration string, threads uint64) (*ClientSession, error) {
	directory, err := os.MkdirTemp("", "isla-client")
	if err != nil {
		return nil, engineError(ProcessFail, "isla-client directory", err.Error())
	}
	socket := filepath.Join(directory, "isla.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		return nil, engineError(ProcessFail, "isla-client socket", err.Error())
	}
	process := exec.CommandContext(ctx, tool,
		"-A", architecture, "-C", configuration,
		"-T", strconv.FormatUint(threads, 10), "--socket", socket)
	if err := process.Start(); err != nil {
		return nil, engineError(ProcessFail, "isla-client start", err.Error())
	}
	connection, err := acceptWithin(listener, 120*time.Second)
	if err != nil {
		return nil, err
	}
	return &ClientSession{process: process, connection: connection,
		listener: listener, directory: directory}, nil
}

// acceptWithin waits for the process to connect, or reports the wait failed.
func acceptWithin(listener net.Listener, limit time.Duration) (net.Conn, error) {
	type accepted struct {
		connection net.Conn
		err        error
	}
	results := make(chan accepted, 1)
	go func() {
		connection, err := listener.Accept()
		results <- accepted{connection, err}
	}()
	select {
	case result := <-results:
		if result.err != nil {
			return nil, engineError(ProcessFail, "isla-client accept", result.err.Error())
		}
		return result.connection, nil
	case <-time.After(limit):
		return nil, engineError(ProcessFail, "isla-client accept", "the process never connected")
	}
}

// Version asks the session which isla build answers on it.
func (session *ClientSession) Version() (string, error) {
	if err := writeMessage(session.connection, "version"); err != nil {
		return "", engineError(ProcessFail, "isla-client version", err.Error())
	}
	tag, err := readAnswerTag(session.connection)
	if err != nil {
		return "", engineError(ProcessFail, "isla-client version", err.Error())
	}
	if tag != answerVersion {
		return "", engineError(ProcessFail, "isla-client version", fmt.Sprintf("answer tag %d", tag))
	}
	body, err := readLengthPrefixed(session.connection)
	if err != nil {
		return "", engineError(ProcessFail, "isla-client version", err.Error())
	}
	return string(body), nil
}

// Execute runs one instruction and returns every trace it produced.
func (session *ClientSession) Execute(encoding string) ([][]byte, error) {
	if err := writeMessage(session.connection, "execute "+encoding); err != nil {
		return nil, engineError(ProcessFail, "isla-client execute", err.Error())
	}
	traces, err := ReadTraces(session.connection)
	if err != nil {
		return nil, engineError(ProcessFail, "isla-client execute "+encoding, err.Error())
	}
	return traces, nil
}

// Close stops the process and removes the socket.
func (session *ClientSession) Close() error {
	writeError := writeMessage(session.connection, "stop")
	_ = session.connection.Close()
	_ = session.listener.Close()
	waitError := session.process.Wait()
	_ = os.RemoveAll(session.directory)
	if writeError != nil {
		return engineError(ProcessFail, "isla-client stop", writeError.Error())
	}
	if waitError != nil {
		return engineError(ProcessFail, "isla-client exit", waitError.Error())
	}
	return nil
}
