package api

// DebuggerState represents the current context of the debugger.
type DebuggerState struct {
	// BreakPoint is the current breakpoint at which the debugged process is
	// suspended, and may be empty if the process is not suspended.
	BreakPoint *BreakPoint `json:"breakPoint,omitempty"`
	// CurrentThread is the currently selected debugger thread.
	CurrentThread *Thread `json:"currentThread,omitempty"`
	// Exited indicates whether the debugged process has exited.
	Exited bool `json:"exited"`
}

// BreakPoint addresses a location at which process execution may be
// suspended.
type BreakPoint struct {
	// ID is a unique identifier for the breakpoint.
	ID int `json:"id"`
	// Addr is the address of the breakpoint.
	Addr uint64 `json:"addr"`
	// File is the source file for the breakpoint.
	File string `json:"file"`
	// Line is a line in File for the breakpoint.
	Line int `json:"line"`
	// FunctionName is the name of the function at the current breakpoint, and
	// may not always be available.
	FunctionName string `json:"functionName,omitempty"`
}

// Thread is a thread within the debugged process.
type Thread struct {
	// ID is a unique identifier for the thread.
	ID int `json:"id"`
	// PC is the current program counter for the thread.
	PC uint64 `json:"pc"`
	// File is the file for the program counter.
	File string `json:"file"`
	// Line is the line number for the program counter.
	Line int `json:"line"`
	// Function is function information at the program counter. May be nil.
	Function *Function `json:"function,omitempty"`
}

// Function represents thread-scoped function information.
type Function struct {
	// Name is the function name.
	Name   string `json:"name"`
	Value  uint64 `json:"value"`
	Type   byte   `json:"type"`
	GoType uint64 `json:"goType"`
	// Args are the function arguments in a thread context.
	Args []Variable `json:"args"`
	// Locals are the thread local variables.
	Locals []Variable `json:"locals"`
}

// Variable describes a variable.
type Variable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Type  string `json:"type"`
}

// Goroutine represents the information relevant to Delve from the runtime's
// internal G structure.
type Goroutine struct {
	// ID is a unique identifier for the goroutine.
	ID int `json:"id"`
	// PC is the current program counter for the goroutine.
	PC uint64 `json:"pc"`
	// File is the file for the program counter.
	File string `json:"file"`
	// Line is the line number for the program counter.
	Line int `json:"line"`
	// Function is function information at the program counter. May be nil.
	Function *Function `json:"function,omitempty"`
}

// DebuggerCommand is a command which changes the debugger's execution state.
type DebuggerCommand struct {
	// Name is the command to run.
	Name string `json:"name"`
	// ThreadID is used to specify which thread to use with the SwitchThread
	// command.
	ThreadID int `json:"threadID,omitempty"`
}

const (
	// Continue resumes process execution.
	Continue = "continue"
	// Step continues for a single instruction, entering function calls.
	Step = "step"
	// Next continues to the next source line, not entering function calls.
	Next = "next"
	// SwitchThread switches the debugger's current thread context.
	SwitchThread = "switchThread"
	// Halt suspends the process.
	Halt = "halt"
)
