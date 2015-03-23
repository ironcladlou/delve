package server

import (
	"fmt"
	"runtime"
	"time"

	"github.com/golang/glog"
	sys "golang.org/x/sys/unix"

	api "github.com/derekparker/delve/api"
	proctl "github.com/derekparker/delve/proctl"
)

type Debugger struct {
	Commands chan *api.Command
	Events   chan *api.Event

	shutdown           chan bool
	fullNotifyInterval time.Duration
	process            *proctl.DebuggedProcess
	commandHandlers    map[api.CommandName]commandHandler
}

type commandHandler func(*api.Command) error

func NewDebugger(shutdown chan bool) *Debugger {
	debugger := &Debugger{
		Commands:           make(chan *api.Command),
		Events:             make(chan *api.Event),
		shutdown:           shutdown,
		fullNotifyInterval: 1 * time.Second,
	}
	debugger.commandHandlers = map[api.CommandName]commandHandler{
		api.AddBreakPoint:    debugger.AddBreakPoint,
		api.Clear:            debugger.Clear,
		api.ClearBreakPoints: debugger.ClearBreakPoints,
		api.Detach:           debugger.Detach,
		api.Kill:             debugger.Kill,
		api.Continue:         debugger.Continue,
		api.Step:             debugger.Step,
		api.Next:             debugger.Next,
	}
	return debugger
}

// TODO: attach
func (d *Debugger) Run(processArgs []string) error {
	// We must ensure here that we are running on the same thread during
	// the execution of dbg. This is due to the fact that ptrace(2) expects
	// all commands after PTRACE_ATTACH to come from the same thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	glog.Infof("launching process with args: %v", processArgs)
	process, err := proctl.Launch(processArgs)
	if err != nil {
		return fmt.Errorf("couldn't launch process: %s", err)
	}
	d.process = process

	ticker := time.NewTicker(d.fullNotifyInterval)

runLoop:
	for {
		select {
		case command := <-d.Commands:
			handler, hasHandler := d.commandHandlers[command.Name]
			if !hasHandler {
				glog.Errorf("no handler for command %s", command.Name)
				continue
			}

			glog.V(1).Infof("handling command: %s", command.Name)
			err = handler(command)
			if err != nil {
				glog.Errorf("handler error: %s", err)
			}
		case <-ticker.C:
			glog.V(5).Info("performing full notify")
			// TODO(danmace): audit concurrency; this stuff should all be reads.
			// TODO(danmace): drive this from events deeper down in proctl.
			d.NotifyBreakPointsUpdated()
			d.NotifyThreadsUpdated()
			d.NotifyProcessUpdated()
		case <-d.shutdown:
			break runLoop
		}
	}
	glog.Info("debugger stopping")
	return nil
}

func (d *Debugger) sendMessage(body string) {
	d.Events <- &api.Event{
		Name: api.Message,
		Message: &api.MessageData{
			Body: body,
		},
	}
}

func (d *Debugger) Detach(command *api.Command) error {
	d.sendMessage("Detaching from process")
	pid := d.process.Process.Pid
	err := sys.PtraceDetach(pid)
	glog.V(0).Infof("attempted detach from %d with result: %v", pid, err)
	return err
}

func (d *Debugger) Kill(command *api.Command) error {
	d.sendMessage("Killing process")
	pid := d.process.Process.Pid
	err := d.process.Process.Kill()
	glog.V(0).Infof("attempted kill of %d with result: %v", pid, err)
	return err
}

func (d *Debugger) ClearBreakPoints(command *api.Command) error {
	if d.process.Exited() {
		return nil
	}

	for _, bp := range d.process.HWBreakPoints {
		if bp == nil {
			continue
		}
		if _, err := d.process.Clear(bp.Addr); err != nil {
			fmt.Printf("Can't clear breakpoint @%x: %s\n", bp.Addr, err)
		}
	}

	for pc := range d.process.BreakPoints {
		if _, err := d.process.Clear(pc); err != nil {
			fmt.Printf("Can't clear breakpoint @%x: %s\n", pc, err)
		}
	}
	d.sendMessage("Cleared all breakpoints")
	return nil
}

func (d *Debugger) AddBreakPoint(command *api.Command) error {
	loc := command.AddBreakPoint.Location

	bp, err := d.process.BreakByLocation(loc)
	if err != nil {
		d.sendMessage(fmt.Sprintf("Error setting breakpoint: %s", err))
	} else {
		d.sendMessage(fmt.Sprintf(
			"Breakpoint %d set at %#v for %s %s:%d", bp.ID, bp.Addr, bp.FunctionName, bp.File, bp.Line))
		d.NotifyBreakPointsUpdated()
	}
	return err
}

func (d *Debugger) Continue(command *api.Command) error {
	if d.process.Exited() {
		return nil
	}
	return d.process.Continue()
}

func (d *Debugger) Step(command *api.Command) error {
	if d.process.Exited() {
		return nil
	}
	return d.process.Step()
}

func (d *Debugger) Next(command *api.Command) error {
	if d.process.Exited() {
		return nil
	}
	return d.process.Next()
}

func (d *Debugger) Clear(command *api.Command) error {
	if d.process.Exited() {
		return nil
	}
	_, err := d.process.Clear(command.Clear.BreakPoint)
	return err
}

func (d *Debugger) NotifyBreakPointsUpdated() error {
	bps := []*api.BreakPoint{}
	for _, bp := range d.process.HWBreakPoints {
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

	for _, bp := range d.process.BreakPoints {
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

	d.Events <- &api.Event{
		Name: api.BreakPointsUpdated,
		BreakPointsUpdated: &api.BreakPointsUpdatedData{
			Timestamp:   time.Now().UnixNano(),
			BreakPoints: bps,
		},
	}

	return nil
}

func (d *Debugger) NotifyThreadsUpdated() error {
	threads := []*api.Thread{}

	for _, th := range d.process.Threads {
		pc, err := d.process.CurrentPC()
		if err != nil {
			// TODO: logging
			continue
		}

		f, l, fn := d.process.GoSymTable.PCToLine(pc)
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
			IsCurrent:   (th.Id == d.process.CurrentThread.Id),
		}

		threads = append(threads, thread)
	}

	d.Events <- &api.Event{
		Name: api.ThreadsUpdated,
		ThreadsUpdated: &api.ThreadsUpdatedData{
			Timestamp: time.Now().UnixNano(),
			Threads:   threads,
		},
	}
	return nil
}

func (d *Debugger) NotifyProcessUpdated() error {
	files := []string{}
	for f := range d.process.GoSymTable.Files {
		files = append(files, f)
	}
	status := d.process.Status()
	statusCode := uint32(0)
	exited := false
	if status != nil {
		glog.Infof("status=%+v", status)
		statusCode = uint32(*status)
		exited = status.Exited()
	} else {
		exited = true
		glog.Info("no status")
	}

	d.Events <- &api.Event{
		Name: api.ProcessUpdated,
		ProcessUpdated: &api.ProcessUpdatedData{
			Process: &api.Process{
				Files:  files,
				Status: statusCode,
				Exited: exited,
			},
		},
	}
	return nil
}
