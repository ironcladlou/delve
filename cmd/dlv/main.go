package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"time"

	sys "golang.org/x/sys/unix"

	"github.com/derekparker/delve/client"
	"github.com/derekparker/delve/proctl/server"
	"github.com/derekparker/delve/proctl/server/websocket"
	"github.com/derekparker/delve/terminal"
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

	launchArgs, err := buildLaunchArgs(os.Args[1:])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	shutdown := make(chan bool)
	wg := sync.WaitGroup{}
	wg.Add(1)

	debugger := server.NewDebugger(shutdown)
	go func() {
		debugger.Run(launchArgs)
		wg.Done()
	}()

	wsServer := &websocket.WebsocketServer{
		Debugger:   debugger,
		ListenAddr: "127.0.0.1",
		ListenPort: 9223,
		Shutdown:   shutdown,
	}
	go func() {
		wsServer.Run()
		//TODO: use an http listener with shutdown support
		// wg.Done()
	}()

	// TODO: fix connection timeout handling
	time.Sleep(500 * time.Millisecond)
	client := client.NewWebsocketClient(wsServer.URL())
	if err := client.Open(); err != nil {
		fmt.Printf("error creating client: %s", err)
		os.Exit(1)
	}

	// handle interrupts
	ch := make(chan os.Signal)
	signal.Notify(ch, sys.SIGINT)
	go func() {
		for range ch {
			// TODO: what should we do here?
			/*
				if process.Running() {
					fmt.Println("Requesting manual stop")
					process.RequestManualStop()
				}
			*/
		}
	}()

	term := terminal.New(client)
	err, status := term.Run()
	if err != nil {
		fmt.Println(err)
	}

	shutdown <- true
	fmt.Print("waiting for debugger and server to shut down...")
	wg.Wait()
	fmt.Println(" done.")

	os.Exit(status)
}

func buildLaunchArgs(args []string) ([]string, error) {
	switch args[0] {
	case "run":
		const debugname = "debug"
		cmd := exec.Command("go", "build", "-o", debugname, "-gcflags", "-N -l")
		err := cmd.Run()
		if err != nil {
			return nil, fmt.Errorf("Could not compile program: %s", err)
		}
		defer os.Remove(debugname)

		return append([]string{"./" + debugname}, args...), nil
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

		return append([]string{debugname}, args...), nil
		/*
			case "attach":
				pid, err := strconv.Atoi(args[1])
				if err != nil {
					return nil, fmt.Errorf("Invalid pid: %d", args[1])
				}
				dbp, err = proctl.Attach(pid)
				if err != nil {
					return nil, fmt.Errorf("Could not attach to process: %s", err)
				}
		*/
	default:
		return args, nil
	}
}
