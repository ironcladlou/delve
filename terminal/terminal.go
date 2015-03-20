package terminal

import (
	"fmt"
	"io"
	"os"
	"strings"

	api "github.com/derekparker/delve/api"
	client "github.com/derekparker/delve/client"
	proctl "github.com/derekparker/delve/proctl"

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
	breakPoints []*api.BreakPoint
	threads     []*api.Thread
}

func New(client client.Interface) *Term {
	return &Term{
		prompt: "(dlv) ",
		line:   liner.NewLiner(),
		client: client,
		cache:  &cache{},
	}
}

func (t *Term) die(status int, args ...interface{}) {
	if t.line != nil {
		t.line.Close()
	}

	fmt.Fprint(os.Stderr, args)
	fmt.Fprint(os.Stderr, "\n")
	os.Exit(status)
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

func (t *Term) Run() {
	defer t.line.Close()

	go t.handleEvents()

	cmds := DebugCommands(t.cache, t.client)
	f, err := os.Open(historyFile)
	if err != nil {
		f, _ = os.Create(historyFile)
	}
	t.line.ReadHistory(f)
	f.Close()
	fmt.Println("Type 'help' for list of commands.")

	for {
		cmdstr, err := t.promptForInput()
		if len(cmdstr) == 0 {
			continue
		}

		if err != nil {
			if err == io.EOF {
				handleExit(t.client, t, 0)
			}
			t.die(1, "Prompt for input failed.\n")
		}

		cmdstr, args := parseCommand(cmdstr)

		if cmdstr == "exit" {
			handleExit(t.client, t, 0)
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
}

func (t *Term) handleEvents() {
	for {
		event, err := t.client.NextEvent()
		if err != nil {
			fmt.Printf("event error: %s", err)
			continue
		}

		fmt.Printf("received client event: %#v\n", event)
		switch event.Name {
		case api.BreakPointsUpdated:
			// TODO: copy
			t.cache.breakPoints = event.BreakPointsUpdated.BreakPoints
		default:
			fmt.Printf("unsupported event %s", event.Name)
		}
	}
}

func handleExit(client client.Interface, t *Term, status int) {
	if f, err := os.OpenFile(historyFile, os.O_RDWR, 0666); err == nil {
		_, err := t.line.WriteHistory(f)
		if err != nil {
			fmt.Println("readline history error: ", err)
		}
		f.Close()
	}

	answer, err := t.line.Prompt("Would you like to kill the process? [y/n]")
	if err != nil {
		t.die(2, io.EOF)
	}
	answer = strings.TrimSuffix(answer, "\n")

	client.ClearBreakPoints()

	fmt.Println("Detaching from process...")
	client.Detach()

	if answer == "y" {
		fmt.Println("Killing process")

		client.Kill()
	}

	t.die(status, "Hope I was of service hunting your bug!")
}

func parseCommand(cmdstr string) (string, []string) {
	vals := strings.Split(cmdstr, " ")
	return vals[0], vals[1:]
}
