// TODO: rename this file protocol.go?
package api

// Command controls the debugger.
type Command struct {
	Name             CommandName           `json:"name"`
	AddBreakPoint    *AddBreakPointCommand `json:"addBreakPoint"`
	ClearBreakPoints *EmptyCommand         `json:"clearBreakPoints"`
	Detach           *EmptyCommand         `json:"detach"`
	Kill             *EmptyCommand         `json:"kill"`
	SwitchThread     *SwitchThreadCommand  `json:"switchThreads"`
	Continue         *EmptyCommand         `json:"continue"`
	Step             *EmptyCommand         `json:"step"`
	Next             *EmptyCommand         `json:"step"`
	Clear            *ClearCommand         `json:"clear"`
}

type CommandName string

const (
	AddBreakPoint    CommandName = "AddBreakPoint"
	ClearBreakPoints CommandName = "ClearBreakPoints"
	Detach           CommandName = "Detach"
	Kill             CommandName = "Kill"
	SwitchThread     CommandName = "SwitchThread"
	Continue         CommandName = "Continue"
	Step             CommandName = "Step"
	Next             CommandName = "Next"
	Clear            CommandName = "Clear"
)

type EmptyCommand struct{}

type AddBreakPointCommand struct {
	Location string `json:"location"`
}

type SwitchThreadCommand struct {
	ID int `json:"id"`
}

type ClearCommand struct {
	BreakPoint uint64 `json:"breakPoint"`
}

// Event is data received from the debugger.
type Event struct {
	Name               EventName               `json:"name"`
	Message            *MessageData            `json:"message,omitempty"`
	ThreadsUpdated     *ThreadsUpdatedData     `json:"threadsUpdated,omitempty"`
	BreakPointsUpdated *BreakPointsUpdatedData `json:"breakPointsUpdated,omitempty"`
	ProcessUpdated     *ProcessUpdatedData     `json:"processUpdated"`
}

type EventName string

const (
	Message            EventName = "Message"
	ThreadsUpdated     EventName = "ThreadsUpdated"
	BreakPointsUpdated EventName = "BreakPointsUpdated"
	ProcessUpdated     EventName = "ProcessUpdated"
)

type EmptyEventData struct{}

type ThreadsUpdatedData struct {
	Timestamp int64     `json:"timestamp"`
	Threads   []*Thread `json:"threads"`
}

type BreakPointsUpdatedData struct {
	Timestamp   int64         `json:"timestamp"`
	BreakPoints []*BreakPoint `json:"breakPoints"`
}

// Not really thought out yet, just need more than nothing.
type MessageData struct {
	Body    string `json:"body"`
	Level   int    `json:"level"`
	IsError bool   `json:"isError"`
}

type ProcessUpdatedData struct {
	Process *Process `json:"process"`
}
