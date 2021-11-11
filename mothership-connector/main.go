package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"t0ast.cc/tbml/mothership-connector/com"
	uerror "t0ast.cc/tbml/util/error"
)

var messaging com.NativeMessagingPort

func main() {
	// Uncomment for debugging; Should be commented in production to
	// avoid writing unnecessary log information to disk.
	// redirectStderr()

	logger := log.Default()

	ctx, _ := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	go func() {
		<-ctx.Done()
		logger.Println("Got SIGTERM")
	}()

	stdin := makeClosable(os.Stdin)
	defer stdin.Close()

	nm, err := com.NewNativeMessagingPort(stdin, os.Stdout)
	uerror.ErrPanic(err)
	messaging = nm

	// Get control socket path
	socketPathMsg, err := receiveMessage()
	errPanicWithLog(err)
	if socketPathMsg.Type != com.MsgTypeInInitControlSocketPath {
		panicWithLog(fmt.Errorf("Initialization error: Wanted message \"%s\" but got \"%s\"", com.MsgTypeInInitControlSocketPath, socketPathMsg.Type))
	}
	controlSocketPath, ok := socketPathMsg.Data.(string)
	if !ok {
		panicWithLog(fmt.Errorf("\"%s\" data was not a string", com.MsgTypeInInitControlSocketPath))
	}
	sendLogMessage(com.MsgTypeOutConnectorLog, fmt.Sprint("Got control socket path:", controlSocketPath))

	// Connect to control socket
	addr, err := net.ResolveUnixAddr("unix", controlSocketPath)
	errPanicWithLog(err)
	sendMessage(com.MsgTypeOutConnectorLog, "Resolved unix domain socket addresss")
	socketConn, err := net.DialUnix("unix", nil, addr)
	errPanicWithLog(err)
	defer socketConn.Close()
	sendMessage(com.MsgTypeOutConnectorLog, "Established unix connection")

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		sc := bufio.NewScanner(socketConn)
	RECEIVE:
		for sc.Scan() {
			select {
			case <-ctx.Done():
				break RECEIVE
			default:
			}

			logger.Println("Not exiting receive")

			var tbmlMsg interface{}
			if err := json.Unmarshal(sc.Bytes(), &tbmlMsg); err != nil {
				sendLogMessage(com.MsgTypeOutConnectorError, fmt.Errorf("Failed to unmarshal incoming JSON message object: %w\n\t%s", err, sc.Bytes()))
				continue
			}
			sendMessage(com.MsgTypeOutTBML, tbmlMsg)
		}

		logger.Println("Exiting receive")
	}()

	go func() {
		defer wg.Done()
	SEND:
		for {
			select {
			case <-ctx.Done():
				break SEND
			default:
			}

			logger.Println("Not exiting send")

			dataMsg, err := receiveMessage()
			if err != nil {
				if !errors.Is(err, os.ErrClosed) {
					sendLogMessage(com.MsgTypeOutConnectorError, fmt.Errorf("Failed to receive outgoing message: %w", err))
				}
				continue
			}
			if dataMsg.Type != com.MsgTypeInTBML {
				sendMessage(com.MsgTypeOutConnectorLog, fmt.Sprintf("WARNING: Skipping message with type \"%s\" (only accepting \"tbml\")", dataMsg.Type))
				continue
			}

			dataBytes, err := json.Marshal(dataMsg.Data)
			errPanicWithLog(err)

			n, err := socketConn.Write(dataBytes)
			errPanicWithLog(err)
			if n != len(dataBytes) {
				panicWithLog(fmt.Errorf("Number of bytes written to Unix socket (%d) != length (%d)", n, len(dataBytes)))
			}
			_, err = socketConn.Write([]byte("\n"))
			errPanicWithLog(err)
		}

		logger.Println("Exiting send")
	}()

	<-ctx.Done()
	_ = (socketConn.Close())
	_ = (stdin.Close())
	wg.Wait()
}

func redirectStderr() {
	f, err := os.Create("mothership-connector-stderr.log")
	uerror.ErrPanic(err)
	uerror.ErrPanic(syscall.Dup2(int(f.Fd()), 2))
}

func makeClosable(reader io.Reader) io.ReadCloser {
	pr, pw := io.Pipe()
	go func() {
		_, _ = io.Copy(pw, reader)
	}()
	return pr
}

func receiveMessage() (com.MsgIn, error) {
	var msg com.MsgIn
	return msg, messaging.ReceiveMessage(&msg)
}

func sendMessage(typ com.MsgTypeOut, data interface{}) {
	uerror.ErrPanic(
		messaging.SendMessage(com.MsgOut{
			Type: typ,
			Data: data,
		}))
}

func errPanicWithLog(data interface{}) {
	if data != nil {
		panicWithLog(data)
	}
}

func panicWithLog(data interface{}) {
	sendLogMessage(com.MsgTypeOutConnectorError, data)
	panic(data)
}

func sendLogMessage(typ com.MsgTypeOut, data interface{}) {
	switch typ := data.(type) {
	case uerror.ErrorWithStackTrace:
		var errStr string
		if typ.Unwrap() != nil {
			errStr = typ.Unwrap().Error()
		}
		data = struct {
			Error      string
			StackTrace string
		}{
			Error:      errStr,
			StackTrace: typ.StackTrace,
		}
	case error:
		data = typ.Error()
	}
	sendMessage(typ, data)
}
