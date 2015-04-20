package rest

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/derekparker/delve/service"
	"github.com/derekparker/delve/service/api"
)

const (
	continuetestprog = "../../_fixtures/continuetestprog"
	testprog         = "../../_fixtures/testprog"
	testnextprog     = "../../_fixtures/testnextprog"
	testthreads      = "../../_fixtures/testthreads"
)

func withTestClient(name string, t *testing.T, fn func(c service.Client)) {
	// Make a (good enough) random temporary file name
	r := make([]byte, 4)
	rand.Read(r)
	file := filepath.Join(os.TempDir(), filepath.Base(name)+hex.EncodeToString(r))

	// Build the test binary
	if err := exec.Command("go", "build", "-gcflags=-N -l", "-o", file, name+".go").Run(); err != nil {
		t.Fatalf("Could not compile %s due to %s", name, err)
	}
	t.Logf("Created test binary %s", file)
	defer os.Remove(file)

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("couldn't start listener: %s\n", err)
	}

	server := NewServer(&Config{
		Listener:    listener,
		ProcessArgs: []string{file},
	})
	go server.Run()

	client := NewClient(listener.Addr().String())
	defer client.Detach(true)

	fn(client)
}

func TestClientServer_exit(t *testing.T) {
	withTestClient(continuetestprog, t, func(c service.Client) {
		state, err := c.Continue()
		if err != nil {
			t.Fatalf("Unexpected error: %v, state: %#v", err, state)
		}
		if state.CurrentThread == nil {
			t.Fatalf("Expected CurrentThread")
		}
		if e, a := 0, state.CurrentThread.State.ExitStatus; e != a {
			t.Fatalf("Expected exit status %d, got %d", e, a)
		}
		if e, a := true, state.CurrentThread.State.Exited; e != a {
			t.Fatalf("Expected exited %v, got %v", e, a)
		}
	})
}

func TestClientServer_step(t *testing.T) {
	withTestClient(testprog, t, func(c service.Client) {
		_, err := c.CreateBreakPoint(&api.BreakPoint{FunctionName: "main.helloworld"})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		stateBefore, err := c.Continue()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		stateAfter, err := c.Step()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if before, after := stateBefore.CurrentThread.PC, stateAfter.CurrentThread.PC; before >= after {
			t.Errorf("Expected %#v to be greater than %#v", before, after)
		}
	})
}

func TestClientServer_next(t *testing.T) {
	testcases := []struct {
		begin, end int
	}{
		{19, 20},
		{20, 23},
		{23, 24},
		{24, 26},
		{26, 31},
		{31, 23},
		{23, 24},
		{24, 26},
		{26, 31},
		{31, 23},
		{23, 24},
		{24, 26},
		{26, 27},
		{27, 34},
		{34, 41},
		{41, 40},
		{40, 41},
	}

	fp, err := filepath.Abs(testnextprog)
	if err != nil {
		t.Fatal(err)
	}
	fp = fp + ".go"

	withTestClient(testnextprog, t, func(c service.Client) {
		bp, err := c.CreateBreakPoint(&api.BreakPoint{File: fp, Line: testcases[0].begin})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		state, err := c.Continue()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		_, err = c.ClearBreakPoint(bp.ID)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		for _, tc := range testcases {
			if state.CurrentThread.Line != tc.begin {
				t.Fatalf("Program not stopped at correct spot expected %d was %s:%d", tc.begin, filepath.Base(fp), state.CurrentThread.Line)
			}

			t.Logf("Next for scenario %#v", tc)
			state, err = c.Next()
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if state.CurrentThread.Line != tc.end {
				t.Fatalf("Program did not continue to correct next location expected %d was %s:%d", tc.end, filepath.Base(fp), state.CurrentThread.Line)
			}
		}
	})
}

func TestClientServer_breakpointInMainThread(t *testing.T) {
	withTestClient(testprog, t, func(c service.Client) {
		bp, err := c.CreateBreakPoint(&api.BreakPoint{FunctionName: "main.helloworld"})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		state, err := c.Continue()
		if err != nil {
			t.Fatalf("Unexpected error: %v, state: %#v", err, state)
		}

		pc := state.CurrentThread.PC

		if pc-1 != bp.Addr && pc != bp.Addr {
			f, l := state.CurrentThread.File, state.CurrentThread.Line
			t.Fatalf("Break not respected:\nPC:%#v %s:%d\nFN:%#v \n", pc, f, l, bp.Addr)
		}
	})
}

func TestClientServer_breakpointInSeparateGoroutine(t *testing.T) {
	withTestClient(testthreads, t, func(c service.Client) {
		_, err := c.CreateBreakPoint(&api.BreakPoint{FunctionName: "main.anotherthread"})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		state, err := c.Continue()
		if err != nil {
			t.Fatalf("Unexpected error: %v, state: %#v", err, state)
		}

		f, l := state.CurrentThread.File, state.CurrentThread.Line
		if f != "testthreads.go" && l != 8 {
			t.Fatal("Program did not hit breakpoint")
		}
	})
}

func TestClientServer_breakAtNonexistentPoint(t *testing.T) {
	withTestClient(testprog, t, func(c service.Client) {
		_, err := c.CreateBreakPoint(&api.BreakPoint{FunctionName: "nowhere"})
		if err == nil {
			t.Fatal("Should not be able to break at non existent function")
		}
	})
}

func TestClientServer_clearBreakpoint(t *testing.T) {
	withTestClient(testprog, t, func(c service.Client) {
		bp, err := c.CreateBreakPoint(&api.BreakPoint{FunctionName: "main.sleepytime"})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		bps, err := c.ListBreakPoints()
		if e, a := 1, len(bps); e != a {
			t.Fatalf("Expected breakpoint count %d, got %d", e, a)
		}

		deleted, err := c.ClearBreakPoint(bp.ID)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if deleted.ID != bp.ID {
			t.Fatalf("Expected deleted breakpoint ID %v, got %v", bp.ID, deleted.ID)
		}

		bps, err = c.ListBreakPoints()
		if e, a := 0, len(bps); e != a {
			t.Fatalf("Expected breakpoint count %d, got %d", e, a)
		}
	})
}

func TestClientServer_switchThread(t *testing.T) {
	withTestClient(testnextprog, t, func(c service.Client) {
		// With invalid thread id
		_, err := c.SwitchThread(-1)
		if err == nil {
			t.Fatal("Expected error for invalid thread id")
		}

		_, err = c.CreateBreakPoint(&api.BreakPoint{FunctionName: "main.main"})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		state, err := c.Continue()
		if err != nil {
			t.Fatalf("Unexpected error: %v, state: %#v", err, state)
		}

		var nt int
		ct := state.CurrentThread.ID
		threads, err := c.ListThreads()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		for _, th := range threads {
			if th.ID != ct {
				nt = th.ID
				break
			}
		}
		if nt == 0 {
			t.Fatal("could not find thread to switch to")
		}
		// With valid thread id
		state, err = c.SwitchThread(nt)
		if err != nil {
			t.Fatal(err)
		}
		if state.CurrentThread.ID != nt {
			t.Fatal("Did not switch threads")
		}
	})
}
