package main_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"t0ast.cc/tbml/mothership-connector/com"
	uerror "t0ast.cc/tbml/util/error"
	uio "t0ast.cc/tbml/util/io"
)

type connectionHandles struct {
	receiveFromUnixSocket func() interface{}
	sendToUnixSocket      func(msg interface{})

	receiveFromStdout func() (com.MsgOut, error)
	sendToStdin       func(typ com.MsgTypeIn, data interface{}) error
}

func setUpMothershipConnector(t *testing.T) (connection connectionHandles, socketPath string, cleanup func(), err error) {
	cleanup = func() {}
	cleanupCount := 0
	deferCleanup := func(fn func()) {
		c := cleanup
		cc := cleanupCount
		cleanup = func() {
			defer c()
			fn()
			fmt.Fprintln(os.Stderr, "Ran cleanup", cc)
		}
		cleanupCount++
	}

	tmpDir := t.TempDir()

	socketPath = filepath.Join(tmpDir, "control-socket")
	addr, err := net.ResolveUnixAddr("unix", socketPath)
	if err != nil {
		return connectionHandles{}, "", cleanup, uerror.WithStackTrace(err)
	}
	listener, err := net.ListenUnix("unix", addr)
	if err != nil {
		return connectionHandles{}, "", cleanup, uerror.WithStackTrace(err)
	}
	deferCleanup(func() {
		assert.NoError(t, listener.Close())
	})

	connectorCmd := exec.Command("../internal/mothership-connector")

	cleanupWaitGroup := sync.WaitGroup{}
	deferCleanup(func() {
		timeout := time.AfterFunc(5*time.Second, func() {
			_ = connectorCmd.Process.Kill()
		})

		cleanupWaitGroup.Wait()
		assert.True(t, timeout.Stop(), "Timeout hit waiting for exit")
	})

	fromUnixSocket := make(chan interface{})
	toUnixSocket := make(chan interface{})
	deferCleanup(func() {
		close(fromUnixSocket)
		close(toUnixSocket)
	})

	receiveFromUnixSocket := func() interface{} {
		return <-fromUnixSocket
	}
	sendToUnixSocket := func(msg interface{}) {
		toUnixSocket <- msg
	}

	handleUnixConnection := func() {
		cleanupWaitGroup.Add(1)
		defer cleanupWaitGroup.Done()

		conn, err := listener.AcceptUnix()
		if !assert.NoError(t, err) {
			return
		}
		defer conn.Close()

		go func() {
			cleanupWaitGroup.Add(1)
			defer cleanupWaitGroup.Done()

			sc := bufio.NewScanner(conn)
			for sc.Scan() {
				var msg interface{}
				assert.NoError(t, json.Unmarshal(sc.Bytes(), &msg))
				fromUnixSocket <- msg
			}
		}()

		for msg := range toUnixSocket {
			msgBytes, err := json.Marshal(msg)
			assert.NoError(t, err)
			_, err = conn.Write(msgBytes)
			assert.NoError(t, err)
			_, err = conn.Write([]byte("\n"))
			assert.NoError(t, err)
		}
	}

	out, err := connectorCmd.StdoutPipe()
	if err != nil {
		return connectionHandles{}, "", cleanup, uerror.WithStackTrace(err)
	}
	in, err := connectorCmd.StdinPipe()
	if err != nil {
		return connectionHandles{}, "", cleanup, uerror.WithStackTrace(err)
	}

	connectorCmd.Stderr = uio.NewPrefixWriter(os.Stdout, "connector-stderr> ")

	messaging, err := com.NewNativeMessagingPort(out, in)
	if err != nil {
		return connectionHandles{}, "", cleanup, uerror.WithStackTrace(err)
	}

	receiveFromStdout := func() (com.MsgOut, error) {
		var msg com.MsgOut
		err := messaging.ReceiveMessage(&msg)
		return msg, err
	}

	sendToStdin := func(typ com.MsgTypeIn, data interface{}) error {
		return messaging.SendMessage(com.MsgIn{
			Type: typ,
			Data: data,
		})
	}

	waitForConnectorExit := func() {
		cleanupWaitGroup.Add(1)
		defer cleanupWaitGroup.Done()

		assert.NoError(t, connectorCmd.Run())
	}

	go handleUnixConnection()

	go waitForConnectorExit()
	deferCleanup(func() {
		assert.NoError(t, connectorCmd.Process.Signal(syscall.SIGTERM))
	})

	return connectionHandles{
		receiveFromUnixSocket: receiveFromUnixSocket,
		sendToUnixSocket:      sendToUnixSocket,

		receiveFromStdout: receiveFromStdout,
		sendToStdin:       sendToStdin,
	}, socketPath, cleanup, nil
}

func TestMothershipConnector(t *testing.T) {
	conn, socketPath, cleanUp, err := setUpMothershipConnector(t)
	defer cleanUp()
	require.NoError(t, err)

	require.NoError(t, conn.sendToStdin(com.MsgTypeInInitControlSocketPath, socketPath))
	for i := 0; i < 3; i++ {
		m, err := conn.receiveFromStdout()
		assert.NoError(t, err)
		fmt.Println(m)
	}

	msgForward := func(data interface{}) {
		assert.NoError(t, conn.sendToStdin(com.MsgTypeInTBML, data))
		actual := conn.receiveFromUnixSocket()
		assert.Equal(t, data, actual)
	}

	msgBack := func(data interface{}) {
		conn.sendToUnixSocket(data)
		actual, err := conn.receiveFromStdout()
		assert.NoError(t, err)
		assert.Equal(t, com.MsgOut{
			Type: com.MsgTypeOutTBML,
			Data: data,
		}, actual)
	}

	msgForward("Hello world")
	msgBack("Hello back :>")
	msgBack(map[string]interface{}{
		"bool":   true,
		"number": float64(1),
		"string": "test",
	})
	msgForward(map[string]interface{}{
		"bool":   true,
		"number": float64(1),
		"string": "test",
	})
	msgBack(float64(1))
	msgForward(float64(1))
	msgForward(true)
	msgBack(false)
}
