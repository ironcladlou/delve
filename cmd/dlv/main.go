package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	sys "golang.org/x/sys/unix"

	client "github.com/derekparker/delve/client"
	proctl "github.com/derekparker/delve/proctl"
	server "github.com/derekparker/delve/proctl/server"
	terminal "github.com/derekparker/delve/terminal"
)

const version string = "0.5.0.beta"

var usage string = fmt.Sprintf(`Delve version %s

flags:
  -v Print version

Invoke with the path to a binary:

  dlv ./path/to/prog

or use the following commands:
  run - Build, run, and attach to program
  test - Build test binary, run and attach to it
  attach - Attach to running process
`, version)

func init() {
	// We must ensure here that we are running on the same thread during
	// the execution of dbg. This is due to the fact that ptrace(2) expects
	// all commands after PTRACE_ATTACH to come from the same thread.
	runtime.LockOSThread()
}

func main() {
	var printv bool

	flag.BoolVar(&printv, "v", false, "Print version number and exit.")
	flag.Parse()

	if flag.NFlag() == 0 && len(flag.Args()) == 0 {
		fmt.Println(usage)
		os.Exit(0)
	}

	if printv {
		fmt.Printf("Delve version: %s\n", version)
		os.Exit(0)
	}

	process, err := makeAndAttach(os.Args[1:])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// handle interrupts
	ch := make(chan os.Signal)
	signal.Notify(ch, sys.SIGINT)
	go func() {
		for range ch {
			if process.Running() {
				fmt.Println("Requesting manual stop")
				process.RequestManualStop()
			}
		}
	}()

	processOps := make(chan proctl.ProcessOp)
	procManager := proctl.ProcessManager{Ops: processOps}
	// start servers
	listenAddr := "127.0.0.1"
	listenPort := 9223

	startServer(listenAddr, listenPort, procManager)
	time.Sleep(500 * time.Millisecond)
	startTerminal(listenAddr, listenPort)

	// begin handling process operation requests
	for {
		select {
		case op := <-processOps:
			op(process)
		}
	}
}

func startServer(listenAddr string, listenPort int, procManager proctl.ProcessManager) {
	fmt.Printf("Server listening on %s:%d\n", listenAddr, listenPort)
	server := server.NewWebsocketServer(procManager, listenAddr, listenPort)
	server.Run()
}

func startTerminal(listenAddr string, listenPort int) {
	addr := fmt.Sprintf("ws://%s:%d", listenAddr, listenPort)
	fmt.Printf("Attaching client to %s\n", addr)

	client := client.NewWebsocketClient(addr)
	if err := client.Open(); err != nil {
		fmt.Printf("error creating client: %s", err)
		os.Exit(1)
	}
	term := terminal.New(client)
	go term.Run()
}

func makeAndAttach(args []string) (*proctl.DebuggedProcess, error) {
	var dbp *proctl.DebuggedProcess
	var err error

	switch args[0] {
	case "run":
		const debugname = "debug"
		cmd := exec.Command("go", "build", "-o", debugname, "-gcflags", "-N -l")
		err := cmd.Run()
		if err != nil {
			return nil, fmt.Errorf("Could not compile program: %s", err)
		}
		defer os.Remove(debugname)

		dbp, err = proctl.Launch(append([]string{"./" + debugname}, args...))
		if err != nil {
			return nil, fmt.Errorf("Could not launch program: %s", err)
		}
	case "test":
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		base := filepath.Base(wd)
		cmd := exec.Command("go", "test", "-c", "-gcflags", "-N -l")
		err = cmd.Run()
		if err != nil {
			return nil, fmt.Errorf("Could not compile program: %s", err)
		}
		debugname := "./" + base + ".test"
		defer os.Remove(debugname)

		dbp, err = proctl.Launch(append([]string{debugname}, args...))
		if err != nil {
			return nil, fmt.Errorf("Could not launch program: %s", err)
		}
	case "attach":
		pid, err := strconv.Atoi(args[1])
		if err != nil {
			return nil, fmt.Errorf("Invalid pid: %d", args[1])
		}
		dbp, err = proctl.Attach(pid)
		if err != nil {
			return nil, fmt.Errorf("Could not attach to process: %s", err)
		}
	default:
		dbp, err = proctl.Launch(args)
		if err != nil {
			return nil, fmt.Errorf("Could not launch program: %s", err)
		}
	}

	return dbp, nil
}
