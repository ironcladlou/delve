// TODO: rename this file protocol.go?
package api

type Command struct {
	Name             CommandName              `json:"name"`
	AddBreakPoint    *AddBreakPointCommand    `json:"addBreakPoint"`
	ClearBreakPoints *ClearBreakPointsCommand `json:"clearBreakPoints"`
	Detach           *DetachCommand           `json:"detach"`
	Kill             *KillCommand             `json:"kill"`
	SwitchThread     *SwitchThreadCommand     `json:"switchThreads"`
	Continue         *ContinueCommand         `json:"continue"`
	Step             *StepCommand             `json:"step"`
	Next             *NextCommand             `json:"step"`
	Clear            *ClearCommand            `json:"clear"`
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

type AddBreakPointCommand struct {
	Location string `json:"location"`
}

type SwitchThreadCommand struct {
	ID int `json:"id"`
}

type ClearCommand struct {
	BreakPoint uint64 `json:"breakPoint"`
}

type ClearBreakPointsCommand struct{}
type DetachCommand struct{}
type KillCommand struct{}
type ContinueCommand struct{}
type StepCommand struct{}
type NextCommand struct{}

type Event struct {
	Name               EventName               `json:"name"`
	ThreadsUpdated     *ThreadsUpdatedData     `json:"threadsUpdated,omitempty"`
	BreakPointsUpdated *BreakPointsUpdatedData `json:"breakPointsUpdated,omitempty"`
	Message            *MessageData            `json:"message,omitempty"`
}

type EventName string

const (
	ThreadsUpdated     EventName = "ThreadsUpdated"
	BreakPointsUpdated EventName = "BreakPointsUpdated"
	Message            EventName = "Message"
)

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
