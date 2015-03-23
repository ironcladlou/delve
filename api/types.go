package api

type Process struct {
	Files  []string `json:"files"`
	Exited bool     `json:"exited"`
	Status uint32   `json:"status"`
}

type Thread struct {
	ID          int     `json:"id"`
	Status      uint32  `json:"status"`
	CurrentPC   uint64  `json:"currentPc"`
	CurrentLine *PCLine `json:"currentLine,omitempty"`
	IsCurrent   bool    `json:"isCurrent,omitempty"`
}

type PCLine struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Value  uint64 `json:"value"`
	Type   byte   `json:"type"`
	Name   string `json:"name"`
	GoType uint64 `json:"goType"`
}

type BreakPoint struct {
	ID           int    `json:"id"`
	FunctionName string `json:"functionName"`
	File         string `json:"file"`
	Line         int    `json:"line"`
	Addr         uint64 `json:"addr"`
}
