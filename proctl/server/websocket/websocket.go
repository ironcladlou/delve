package websocket

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang/glog"
	gws "github.com/gorilla/websocket"

	"github.com/derekparker/delve/api"
	"github.com/derekparker/delve/proctl/server"
)

type WebsocketServer struct {
	ListenAddr string
	ListenPort int
	Debugger   *server.Debugger
	Shutdown   chan bool
}

func (s *WebsocketServer) Run() error {
	http.HandleFunc("/", s.handleSocket)
	// TODO: hack in shutdown based on a channel
	// TODO: could serialize access with a channel
	glog.Infof("websocket server listening at %s\n", s.URL())
	err := http.ListenAndServe(fmt.Sprintf("%s:%d", s.ListenAddr, s.ListenPort), nil)
	if err != nil {
		return err
	}

	for {
		select {
		case <-s.Shutdown:
			glog.Info("websocket server stopping")
			return nil
		}
	}
}

func (s *WebsocketServer) URL() string {
	return fmt.Sprintf("ws://%s:%d", s.ListenAddr, s.ListenPort)
}

func (s *WebsocketServer) handleSocket(w http.ResponseWriter, r *http.Request) {
	upgrader := gws.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// TODO: error handling
		glog.Errorf("error upgrading connection: %v", err)
		return
	}

	// TODO: stop these when the connection dies
	go s.readCommands(conn)
	go s.writeEvents(conn)
}

func (s *WebsocketServer) readCommands(conn *gws.Conn) {
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			return
		}

		if messageType != gws.TextMessage {
			glog.Errorf("received invalid message type %s", messageType)
			continue
		}

		dec := json.NewDecoder(strings.NewReader(string(message)))

		var command *api.Command
		if err := dec.Decode(&command); err != nil {
			glog.Errorf("couldn't decode command: %s", err)
			continue
		}

		s.Debugger.Commands <- command
	}
}

func (s *WebsocketServer) writeEvents(conn *gws.Conn) {
	for {
		select {
		case event := <-s.Debugger.Events:
			json, err := json.Marshal(event)
			if err != nil {
				glog.Errorf("error marshalling event: %s", err)
				continue
			}
			if err := conn.WriteMessage(gws.TextMessage, json); err != nil {
				glog.Errorf("error writing event: %s", err)
			}
		}
	}
}
