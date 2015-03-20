package api

type Command struct {
	Name             CommandName              `json:"name"`
	AddBreakPoint    *AddBreakPointCommand    `json:"addBreakPoint"`
	ClearBreakPoints *ClearBreakPointsCommand `json:"clearBreakPoints"`
	Detach           *DetachCommand           `json:"detach"`
	Kill             *KillCommand             `json:"kill"`
}

type CommandName string

const (
	AddBreakPoint    CommandName = "AddBreakPoint"
	ClearBreakPoints CommandName = "ClearBreakPoints"
	Detach           CommandName = "Detach"
	Kill             CommandName = "Kill"
)

type AddBreakPointCommand struct {
	Location string `json:"location"`
}

type ClearBreakPointsCommand struct{}
type DetachCommand struct{}
type KillCommand struct{}

type Event struct {
	Name               EventName               `json:"name"`
	ThreadsUpdated     *ThreadsUpdatedData     `json:"threadsUpdated,omitempty"`
	BreakPointsUpdated *BreakPointsUpdatedData `json:"breakPointsUpdated,omitempty"`
}

type EventName string

const (
	ThreadsUpdated     EventName = "ThreadsUpdated"
	BreakPointsUpdated EventName = "BreakPointsUpdated"
)

type ThreadsUpdatedData struct {
	Timestamp int64     `json:"timestamp"`
	Threads   []*Thread `json:"threads"`
}

type BreakPointsUpdatedData struct {
	Timestamp   int64         `json:"timestamp"`
	BreakPoints []*BreakPoint `json:"breakPoints"`
}
