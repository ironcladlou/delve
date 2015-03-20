package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	websocket "github.com/gorilla/websocket"
	sys "golang.org/x/sys/unix"

	api "github.com/derekparker/delve/api"
	proctl "github.com/derekparker/delve/proctl"
)

type WebsocketServer struct {
	listenAddr      string
	listenPort      int
	procManager     proctl.ProcessManager
	commandHandlers map[api.CommandName]commandHandler
	events          chan *api.Event
}

type commandHandler func(proctl.ProcessManager, *api.Command, chan *api.Event) error

func NewWebsocketServer(procManager proctl.ProcessManager, listenAddr string, listenPort int) *WebsocketServer {
	return &WebsocketServer{
		procManager: procManager,
		listenAddr:  listenAddr,
		listenPort:  listenPort,
		events:      make(chan *api.Event),
		commandHandlers: map[api.CommandName]commandHandler{
			api.AddBreakPoint:    handleAddBreakPoint,
			api.ClearBreakPoints: handleClearBreakPoints,
			api.Detach:           handleDetach,
			api.Kill:             handleKill,
		},
	}
}

func (s *WebsocketServer) Run() {
	http.HandleFunc("/", s.handleSocket)
	go func() {
		// TODO: hack in shutdown based on a channel
		// TODO: could serialize access with a channel
		err := http.ListenAndServe(fmt.Sprintf("%s:%d", s.listenAddr, s.listenPort), nil)
		if err != nil {
			fmt.Printf("error starting server: %s", err)
		}
	}()
}

func (s *WebsocketServer) handleSocket(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}

	// TODO: graceful shutdown
	go s.readCommands(conn)
	go s.writeEvents(conn)
}

func (s *WebsocketServer) readCommands(conn *websocket.Conn) {
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			return
		}

		if messageType != websocket.TextMessage {
			// TODO: error handling
			continue
		}

		dec := json.NewDecoder(strings.NewReader(string(message)))

		var command *api.Command
		if err := dec.Decode(&command); err != nil {
			// TODO: error handling
			fmt.Println(err)
			continue
		}

		// Dispatch the command
		handler, hasHandler := s.commandHandlers[command.Name]
		if !hasHandler {
			// TODO: error handling
			fmt.Printf("no handler for command %s\n", command.Name)
			continue
		}

		err = handler(s.procManager, command, s.events)
		if err != nil {
			// TODO: error handling
			fmt.Printf("handler error: %s\n", err)
		}
	}
}

func (s *WebsocketServer) writeEvents(conn *websocket.Conn) {
	for {
		select {
		case event := <-s.events:
			json, err := json.Marshal(event)
			if err != nil {
				fmt.Printf("error marshalling event: %s", err)
				continue
			}
			if err := conn.WriteMessage(websocket.TextMessage, json); err != nil {
				// TODO: error handling
				fmt.Printf("error writing event: %s\n", err)
			}
		}
	}
}

func handleDetach(procManager proctl.ProcessManager, command *api.Command, events chan *api.Event) error {
	err := procManager.Exec(func(proc *proctl.DebuggedProcess) error {
		if proc.Exited() {
			return nil
		}
		return sys.PtraceDetach(proc.Process.Pid)
	})
	return err
}

func handleKill(procManager proctl.ProcessManager, command *api.Command, events chan *api.Event) error {
	err := procManager.Exec(func(proc *proctl.DebuggedProcess) error {
		if proc.Exited() {
			return nil
		}
		return proc.Process.Kill()
	})
	return err
}

func handleClearBreakPoints(procManager proctl.ProcessManager, command *api.Command, events chan *api.Event) error {
	err := procManager.Exec(func(proc *proctl.DebuggedProcess) error {
		if proc.Exited() {
			return nil
		}

		for _, bp := range proc.HWBreakPoints {
			if bp == nil {
				continue
			}
			if _, err := proc.Clear(bp.Addr); err != nil {
				fmt.Printf("Can't clear breakpoint @%x: %s\n", bp.Addr, err)
			}
		}

		for pc := range proc.BreakPoints {
			if _, err := proc.Clear(pc); err != nil {
				fmt.Printf("Can't clear breakpoint @%x: %s\n", pc, err)
			}
		}
		return nil
	})
	return err
}

func handleAddBreakPoint(procManager proctl.ProcessManager, command *api.Command, events chan *api.Event) error {
	loc := command.AddBreakPoint.Location

	err := procManager.Exec(func(proc *proctl.DebuggedProcess) error {
		_, err := proc.BreakByLocation(loc)
		return err
	})

	if err != nil {
		return err
	}

	notifyBreakPointsUpdated(procManager, events)
	return nil
}

func notifyBreakPointsUpdated(procManager proctl.ProcessManager, events chan *api.Event) error {
	err := procManager.Exec(func(proc *proctl.DebuggedProcess) error {
		bps := []*api.BreakPoint{}
		for _, bp := range proc.HWBreakPoints {
			if bp == nil {
				continue
			}
			bps = append(bps, &api.BreakPoint{
				ID:           bp.ID,
				FunctionName: bp.FunctionName,
				File:         bp.File,
				Line:         bp.Line,
				Addr:         bp.Addr,
			})
		}

		for _, bp := range proc.BreakPoints {
			if bp.Temp {
				continue
			}
			bps = append(bps, &api.BreakPoint{
				ID:           bp.ID,
				FunctionName: bp.FunctionName,
				File:         bp.File,
				Line:         bp.Line,
				Addr:         bp.Addr,
			})
		}

		events <- &api.Event{
			Name: api.BreakPointsUpdated,
			BreakPointsUpdated: &api.BreakPointsUpdatedData{
				Timestamp:   time.Now().UnixNano(),
				BreakPoints: bps,
			},
		}

		return nil
	})

	return err
}

func notifyThreadsUpdated(processOps chan proctl.ProcessOp) *api.Event {
	/*
		threads := []*api.Thread{}

		for _, th := range proc.Threads {
			pc, err := proc.CurrentPC()
			if err != nil {
				// TODO: logging
				continue
			}

			f, l, fn := proc.GoSymTable.PCToLine(pc)
			var line *api.PCLine
			if fn != nil {
				line = &api.PCLine{
					File:   f,
					Line:   l,
					Name:   fn.Name,
					Type:   fn.Type,
					Value:  fn.Value,
					GoType: fn.GoType,
				}
			}

			var status uint32
			if th.Status != nil {
				status = uint32(*th.Status)
			}

			thread := &api.Thread{
				ID:          th.Id,
				CurrentLine: line,
				CurrentPC:   pc,
				Status:      status,
				IsCurrent:   (th.Id == proc.CurrentThread.Id),
			}

			threads = append(threads, thread)
		}

		return &api.Event{
			Name: api.ThreadsUpdated,
			ThreadsUpdated: &api.ThreadsUpdatedData{
				Timestamp: time.Now().UnixNano(),
				Threads:   threads,
			},
		}
	*/
	return nil
}
