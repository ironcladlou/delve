package terminal

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"

	api "github.com/derekparker/delve/api"
	client "github.com/derekparker/delve/client"
	proctl "github.com/derekparker/delve/proctl"
	sys "golang.org/x/sys/unix"

	"github.com/peterh/liner"
)

const historyFile string = ".dbg_history"

type Term struct {
	client client.Interface
	prompt string
	line   *liner.State
	cache  *cache
}

type cache struct {
	process     *api.Process
	breakPoints []*api.BreakPoint
	threads     []*api.Thread
}

func New(client client.Interface) *Term {
	return &Term{
		prompt: "(dlv) ",
		line:   liner.NewLiner(),
		client: client,
		cache: &cache{
			process: &api.Process{},
		},
	}
}

func (t *Term) promptForInput() (string, error) {
	l, err := t.line.Prompt(t.prompt)
	if err != nil {
		return "", err
	}

	l = strings.TrimSuffix(l, "\n")
	if l != "" {
		t.line.AppendHistory(l)
	}

	return l, nil
}

func (t *Term) Run() (error, int) {
	defer t.line.Close()

	stop := make(chan bool)
	eventConsumerWg, eventErr := t.consumeEvents(stop)
	if eventErr != nil {
		return fmt.Errorf("Couldn't start event consumer: %s", eventErr), 1
	}

	cmds := DebugCommands(t.cache, t.client)
	f, err := os.Open(historyFile)
	if err != nil {
		f, _ = os.Create(historyFile)
	}
	t.line.ReadHistory(f)
	f.Close()
	fmt.Println("Type 'help' for list of commands.")

	var status int

	for {
		cmdstr, err := t.promptForInput()
		if len(cmdstr) == 0 {
			continue
		}

		if err != nil {
			if err == io.EOF {
				err, status = handleExit(t.client, t)
			}
			err, status = fmt.Errorf("Prompt for input failed.\n"), 1
			break
		}

		cmdstr, args := parseCommand(cmdstr)

		if cmdstr == "exit" {
			err, status = handleExit(t.client, t)
			break
		}

		cmd := cmds.Find(cmdstr)
		if err := cmd(t.client, t.cache, args...); err != nil {
			switch err.(type) {
			case proctl.ProcessExitedError:
				pe := err.(proctl.ProcessExitedError)
				fmt.Fprintf(os.Stderr, "Process exited with status %d\n", pe.Status)
			default:
				fmt.Fprintf(os.Stderr, "Command failed: %s\n", err)
			}
		}
	}

	fmt.Println("Waiting for event consumer to stop...")
	stop <- true
	eventConsumerWg.Wait()

	fmt.Println("Terminal stopped")
	return nil, status
}

func (t *Term) consumeEvents(stop chan bool) (*sync.WaitGroup, error) {
	events, err := t.client.Events()
	if err != nil {
		return nil, fmt.Errorf("couldn't get client event channel: %s\n", err)
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)

	go func() {
		for {
			select {
			case event := <-events:
				switch event.Name {
				case api.Message:
					fmt.Printf("** %s\n", event.Message.Body)
				case api.BreakPointsUpdated:
					// TODO(danmace): copy
					t.cache.breakPoints = event.BreakPointsUpdated.BreakPoints
				case api.ThreadsUpdated:
					// TODO(danmace): copy
					t.cache.threads = event.ThreadsUpdated.Threads
				case api.ProcessUpdated:
					t.cache.process = event.ProcessUpdated.Process
				default:
					fmt.Printf("unsupported event %s\n", event.Name)
				}
			case <-stop:
				wg.Done()
				return
			}
		}
	}()

	return wg, nil
}

func handleExit(client client.Interface, t *Term) (error, int) {
	if f, err := os.OpenFile(historyFile, os.O_RDWR, 0666); err == nil {
		_, err := t.line.WriteHistory(f)
		if err != nil {
			fmt.Println("readline history error: ", err)
		}
		f.Close()
	}

	answer, err := t.line.Prompt("Would you like to kill the process? [y/n]")
	if err != nil {
		return io.EOF, 2
	}
	answer = strings.TrimSuffix(answer, "\n")

	client.ClearBreakPoints()
	client.Detach()

	if answer == "y" {
		client.Kill()
	}

	cancel := make(chan os.Signal)
	signal.Notify(cancel, sys.SIGINT)
	fmt.Println("Waiting for process to terminate (ctrl-c to give up)...")
waitLoop:
	for {
		if t.cache.process.Exited {
			break
		}
		select {
		case <-cancel:
			break waitLoop
		}
	}

	return nil, 0
}

func parseCommand(cmdstr string) (string, []string) {
	vals := strings.Split(cmdstr, " ")
	return vals[0], vals[1:]
}
