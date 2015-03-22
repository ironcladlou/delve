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
	events          chan *api.Event
	resyncInterval  time.Duration
	debugger        *Debugger
	commandHandlers map[api.CommandName]commandHandler
}

type commandHandler func(*api.Command) error

func NewWebsocketServer(procManager proctl.ProcessManager, listenAddr string, listenPort int) *WebsocketServer {
	events := make(chan *api.Event)
	debugger := &Debugger{
		procManager: procManager,
		events:      events,
	}
	return &WebsocketServer{
		listenAddr: listenAddr,
		listenPort: listenPort,
		events:     events,
		// TODO(danmace): value is probably insane, but nice for developing at the
		// moment.
		resyncInterval: 1 * time.Second,
		debugger:       debugger,
		commandHandlers: map[api.CommandName]commandHandler{
			api.AddBreakPoint:    debugger.AddBreakPoint,
			api.Clear:            debugger.Clear,
			api.ClearBreakPoints: debugger.ClearBreakPoints,
			api.Detach:           debugger.Detach,
			api.Kill:             debugger.Kill,
			api.Continue:         debugger.Continue,
			api.Step:             debugger.Step,
			api.Next:             debugger.Next,
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
	go s.resync(conn)
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

		err = handler(command)
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

// TODO(danmace): audit concurrency; this stuff should all be reads.
// TODO(danmace): drive this from events deeper down in proctl.
func (s *WebsocketServer) resync(conn *websocket.Conn) {
	ticker := time.NewTicker(s.resyncInterval)
	for {
		select {
		case <-ticker.C:
			s.debugger.NotifyBreakPointsUpdated()
			s.debugger.NotifyThreadsUpdated()
			// TODO(danmace): this should only need to happen once or on demand.
			s.debugger.NotifyFilesUpdated()
		}
	}
}

type Debugger struct {
	procManager proctl.ProcessManager
	events      chan *api.Event
}

func (d *Debugger) sendMessage(body string) {
	d.events <- &api.Event{
		Name: api.Message,
		Message: &api.MessageData{
			Body: body,
		},
	}
}

func (d *Debugger) Detach(command *api.Command) error {
	return d.procManager.Exec(func(proc *proctl.DebuggedProcess) error {
		if proc.Exited() {
			return nil
		}
		d.sendMessage("Detaching from process")
		return sys.PtraceDetach(proc.Process.Pid)
	})
}

func (d *Debugger) Kill(command *api.Command) error {
	return d.procManager.Exec(func(proc *proctl.DebuggedProcess) error {
		if proc.Exited() {
			return nil
		}
		d.sendMessage("Killing process")
		return proc.Process.Kill()
	})
}

func (d *Debugger) ClearBreakPoints(command *api.Command) error {
	return d.procManager.Exec(func(proc *proctl.DebuggedProcess) error {
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
		d.sendMessage("Cleared all breakpoints")
		return nil
	})
}

func (d *Debugger) AddBreakPoint(command *api.Command) error {
	loc := command.AddBreakPoint.Location

	err := d.procManager.Exec(func(proc *proctl.DebuggedProcess) error {
		bp, err := proc.BreakByLocation(loc)
		d.sendMessage(fmt.Sprintf(
			"Breakpoint %d set at %#v for %s %s:%d\n", bp.ID, bp.Addr, bp.FunctionName, bp.File, bp.Line))
		return err
	})

	d.NotifyBreakPointsUpdated()
	return err
}

func (d *Debugger) Continue(command *api.Command) error {
	return d.procManager.Exec(func(proc *proctl.DebuggedProcess) error {
		if proc.Exited() {
			return nil
		}
		return proc.Continue()
	})
}

func (d *Debugger) Step(command *api.Command) error {
	return d.procManager.Exec(func(proc *proctl.DebuggedProcess) error {
		if proc.Exited() {
			return nil
		}
		return proc.Step()
	})
}

func (d *Debugger) Next(command *api.Command) error {
	return d.procManager.Exec(func(proc *proctl.DebuggedProcess) error {
		if proc.Exited() {
			return nil
		}
		return proc.Next()
	})
}

func (d *Debugger) Clear(command *api.Command) error {
	return d.procManager.Exec(func(proc *proctl.DebuggedProcess) error {
		if proc.Exited() {
			return nil
		}
		_, err := proc.Clear(command.Clear.BreakPoint)
		return err
	})
}

func (d *Debugger) NotifyBreakPointsUpdated() error {
	return d.procManager.Exec(func(proc *proctl.DebuggedProcess) error {
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

		d.events <- &api.Event{
			Name: api.BreakPointsUpdated,
			BreakPointsUpdated: &api.BreakPointsUpdatedData{
				Timestamp:   time.Now().UnixNano(),
				BreakPoints: bps,
			},
		}

		return nil
	})
}

func (d *Debugger) NotifyThreadsUpdated() error {
	return d.procManager.Exec(func(proc *proctl.DebuggedProcess) error {
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

		d.events <- &api.Event{
			Name: api.ThreadsUpdated,
			ThreadsUpdated: &api.ThreadsUpdatedData{
				Timestamp: time.Now().UnixNano(),
				Threads:   threads,
			},
		}
		return nil
	})
}

func (d *Debugger) NotifyFilesUpdated() error {
	return d.procManager.Exec(func(proc *proctl.DebuggedProcess) error {
		files := []string{}
		for f := range proc.GoSymTable.Files {
			files = append(files, f)
		}
		d.events <- &api.Event{
			Name: api.FilesUpdated,
			FilesUpdated: &api.FilesUpdatedData{
				Files: files,
			},
		}
		return nil
	})
}
