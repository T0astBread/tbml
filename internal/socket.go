package internal

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"

	uerror "t0ast.cc/tbml/util/error"
)

type broadcastChannelOpenEvent struct {
	connectionID int
	channel      chan interface{}
}

type broadcastChannelCloseEvent struct {
	connectionID int
}

type openTabBroadcast struct {
	URL string
}

type openedStartURLBroadcast struct{}

type startURLBroadcast struct {
	startURL *url.URL
}

type socketMsgType string

const (
	socketMsgTypeOpenedTab socketMsgType = "opened-tab"
	socketMsgTypeOpenTab   socketMsgType = "open-tab"
)

func ListenOnExternalUnixSocket(ctx context.Context, listener *net.UnixListener, startURL *url.URL) {
	incomingBroadcasts := make(chan interface{})
	newBroadcastChannels := make(chan broadcastChannelOpenEvent)
	closedBroadcastChannels := make(chan broadcastChannelCloseEvent)
	go func() {
		outgoingBroadcasts := make(map[int]chan interface{})
		startURL := startURL
		for {
			select {
			case newBC := <-newBroadcastChannels:
				outgoingBroadcasts[newBC.connectionID] = newBC.channel
				newBC.channel <- startURLBroadcast{
					startURL: startURL,
				}
			case closedBC := <-closedBroadcastChannels:
				delete(outgoingBroadcasts, closedBC.connectionID)

			case broadcast := <-incomingBroadcasts:
				switch broadcast.(type) {
				case openedStartURLBroadcast:
					startURL = nil
				}
				for _, bc := range outgoingBroadcasts {
					bc <- broadcast
				}

			case <-ctx.Done():
				break
			}
		}
	}()

	for nextConnectionID := 0; ; nextConnectionID++ {
		connectionID := nextConnectionID

		select {
		case <-ctx.Done():
			break
		default:
		}

		conn, err := listener.AcceptUnix()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				break
			}
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		outgoingBroadcasts := make(chan interface{})
		newBroadcastChannels <- broadcastChannelOpenEvent{
			connectionID: connectionID,
			channel:      outgoingBroadcasts,
		}
		go func() {
			defer func() {
				closedBroadcastChannels <- broadcastChannelCloseEvent{
					connectionID: connectionID,
				}
				close(outgoingBroadcasts)
			}()
			defer conn.Close()
			if err := handleConnection(ctx, incomingBroadcasts, outgoingBroadcasts, conn); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}()
	}
}

func handleConnection(ctx context.Context, outgoingBroadcasts, incomingBroadcasts chan interface{}, conn *net.UnixConn) error {
	isMothershipConnector := false
	var startURL *url.URL

	ctx, cancelProcessing := context.WithCancel(ctx)
	incomingMsgs := make(chan interface{})
	receiveErrs := make(chan error)

	sc := bufio.NewScanner(conn)
	go func() {
		for sc.Scan() {
			var msg interface{}
			if err := json.Unmarshal(sc.Bytes(), &msg); err != nil {
				receiveErrs <- uerror.WithStackTrace(err)
			}
			incomingMsgs <- msg
		}
		close(incomingMsgs)
		close(receiveErrs)
		cancelProcessing()
	}()

EVENTS:
	for {
		select {
		case broadcast := <-incomingBroadcasts:
			switch broadcast := broadcast.(type) {
			case openTabBroadcast:
				if isMothershipConnector {
					if err := SendOpenTabMessage(conn, broadcast.URL); err != nil {
						return uerror.WithStackTrace(err)
					}
				}
			case openedStartURLBroadcast:
				startURL = nil
			case startURLBroadcast:
				startURL = broadcast.startURL
				if err := openStartURLIfNecessary(conn, startURL, isMothershipConnector); err != nil {
					return uerror.WithStackTrace(err)
				}
			}

		case msg := <-incomingMsgs:
			if msg == "Hello from Mothership! :>" {
				isMothershipConnector = true
				if err := openStartURLIfNecessary(conn, startURL, isMothershipConnector); err != nil {
					return uerror.WithStackTrace(err)
				}
			} else if msg, ok := msg.(map[string]interface{}); ok {
				switch msg["type"] {
				case string(socketMsgTypeOpenTab):
					url, _ := msg["url"].(string)
					outgoingBroadcasts <- openTabBroadcast{
						URL: url,
					}
				case string(socketMsgTypeOpenedTab):
					url, _ := msg["url"].(string)
					if startURL != nil && url == startURL.String() {
						outgoingBroadcasts <- openedStartURLBroadcast{}
					}
				}
			}
		case err := <-receiveErrs:
			return uerror.WithStackTrace(err)

		case <-ctx.Done():
			break EVENTS
		}
	}

	return nil
}

func openStartURLIfNecessary(conn *net.UnixConn, startURL *url.URL, isMothershipConnector bool) error {
	if isMothershipConnector && startURL != nil {
		if err := SendOpenTabMessage(conn, startURL.String()); err != nil {
			return uerror.WithStackTrace(err)
		}
	}
	return nil
}

func ConnectToExternalUnixSocket(config Configuration, instance ProfileInstance) (*net.UnixConn, error) {
	instanceDir := getInstanceDir(config, instance)

	addr, err := resolveExternalUnixSocketAddr(instanceDir)
	if err != nil {
		return nil, uerror.WithStackTrace(err)
	}

	conn, err := net.DialUnix("unix", nil, addr)
	if err != nil {
		return nil, uerror.WithStackTrace(err)
	}

	return conn, nil
}

func SendOpenTabMessage(conn *net.UnixConn, url string) error {
	return sendMessageOverSocket(conn, map[string]interface{}{
		"type": socketMsgTypeOpenTab,
		"url":  url,
	})
}

func resolveExternalUnixSocketAddr(instanceDir string) (*net.UnixAddr, error) {
	addr, err := net.ResolveUnixAddr("unix", filepath.Join(instanceDir, "control-socket"))
	if err != nil {
		return nil, uerror.WithStackTrace(err)
	}
	return addr, nil
}

func sendMessageOverSocket(conn *net.UnixConn, msg interface{}) error {
	resp, err := json.Marshal(msg)
	if err != nil {
		return uerror.WithStackTrace(err)
	}
	if _, err := conn.Write(resp); err != nil {
		return uerror.WithStackTrace(err)
	}
	if _, err := conn.Write([]byte("\n")); err != nil {
		return uerror.WithStackTrace(err)
	}
	return nil
}
