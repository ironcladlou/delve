package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	websocket "github.com/gorilla/websocket"

	api "github.com/derekparker/delve/api"
)

// Interface represents a client connection to the debugging server.
type Interface interface {
	// Open establishes a connection to the debugger.
	Open() error
	// Close closes the connection to the debugger.
	Close() error
	// Events provides a debugger event channel.
	Events() (chan *api.Event, error)
	// AddBreakPoint adds a breakpoint at location.
	AddBreakPoint(location string) error
	// ClearBreakPoint clears all existing breakpoints.
	ClearBreakPoints() error
	// Detach detaches the debugger from the process.
	Detach() error
	// Kill kills the process being debugged.
	Kill() error
	// SwitchThread switches the current debugger thread.
	SwitchThread(ID int) error
	// Continue resumes process execution.
	Continue() error
	// Step steps through the process.
	Step() error
	// Next steps over function calls.
	Next() error
	// Clear clears the breakpoint at addr.
	Clear(addr uint64) error
}

var _ = Interface(&WebsocketClient{})

// WebsockerClient communicates with the debugger via WebSockets.
// Create a WebsocketClient using NewWebsocketClient.
type WebsocketClient struct {
	addr string
	conn *websocket.Conn
}

func NewWebsocketClient(addr string) *WebsocketClient {
	return &WebsocketClient{addr: addr}
}

func (c *WebsocketClient) writeMessage(obj interface{}) error {
	json, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("error marshalling obj: %s", err)
	}
	if err := c.conn.WriteMessage(websocket.TextMessage, json); err != nil {
		return fmt.Errorf("error writing obj: %s", err)
	}
	return nil
}

func (c *WebsocketClient) Open() error {
	dialer := &websocket.Dialer{
		HandshakeTimeout: 3 * time.Second,
		ReadBufferSize:   1024,
		WriteBufferSize:  1024,
	}
	conn, resp, err := dialer.Dial(c.addr, http.Header{})
	if err != nil {
		// TODO: error handling
		return fmt.Errorf("dial error: %s\nresponse:%+v", err, resp)
	}
	c.conn = conn
	return nil
}

func (c *WebsocketClient) Close() error {
	return c.conn.Close()
}

func (c *WebsocketClient) Events() (chan *api.Event, error) {
	events := make(chan *api.Event)
	go func() {
		for {
			messageType, message, err := c.conn.ReadMessage()
			if err != nil {
				// TODO: logging
				fmt.Printf("event reader stopped: %s\n", err)
				close(events)
				return
			}

			if messageType != websocket.TextMessage {
				// TODO: error handling
				fmt.Printf("invalid message type %s\n", messageType)
			}

			dec := json.NewDecoder(strings.NewReader(string(message)))

			var event *api.Event
			if err := dec.Decode(&event); err != nil {
				// TODO: error handling
				fmt.Printf("couldn't decode event: %s\n", err)
			}

			events <- event
		}
	}()
	return events, nil
}

func (c *WebsocketClient) AddBreakPoint(location string) error {
	return c.writeMessage(&api.Command{
		Name: api.AddBreakPoint,
		AddBreakPoint: &api.AddBreakPointCommand{
			Location: location,
		},
	})
}

func (c *WebsocketClient) ClearBreakPoints() error {
	return c.writeMessage(&api.Command{
		Name: api.ClearBreakPoints,
	})
}

func (c *WebsocketClient) Detach() error {
	return c.writeMessage(&api.Command{
		Name: api.Detach,
	})
}

func (c *WebsocketClient) Kill() error {
	return c.writeMessage(&api.Command{
		Name: api.Kill,
	})
}

func (c *WebsocketClient) SwitchThread(ID int) error {
	return c.writeMessage(&api.Command{
		Name: api.SwitchThread,
		SwitchThread: &api.SwitchThreadCommand{
			ID: ID,
		},
	})
}

func (c *WebsocketClient) Continue() error {
	return c.writeMessage(&api.Command{
		Name: api.Continue,
	})
}

func (c *WebsocketClient) Step() error {
	return c.writeMessage(&api.Command{
		Name: api.Step,
	})
}

func (c *WebsocketClient) Next() error {
	return c.writeMessage(&api.Command{
		Name: api.Next,
	})
}

func (c *WebsocketClient) Clear(addr uint64) error {
	return c.writeMessage(&api.Command{
		Name: api.Clear,
		Clear: &api.ClearCommand{
			BreakPoint: addr,
		},
	})
}
