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

type Interface interface {
	Open() error
	Close() error
	AddBreakPoint(location string) error
	ClearBreakPoints() error
	Detach() error
	Kill() error
	NextEvent() (*api.Event, error)
}

var _ = Interface(&WebsocketClient{})

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

func (c *WebsocketClient) NextEvent() (*api.Event, error) {
	messageType, message, err := c.conn.ReadMessage()
	if err != nil {
		return nil, err
	}

	if messageType != websocket.TextMessage {
		// TODO: error handling
		return nil, fmt.Errorf("invalid message type %s", messageType)
	}

	dec := json.NewDecoder(strings.NewReader(string(message)))

	var event *api.Event
	if err := dec.Decode(&event); err != nil {
		return nil, err
	}

	return event, nil
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
	return nil
}

func (c *WebsocketClient) Detach() error {
	return nil
}

func (c *WebsocketClient) Kill() error {
	return nil
}
