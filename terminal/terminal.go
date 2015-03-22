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

	// TODO(danmace): client should return a channel for events
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
				return handleExit(t.client, t)
			}
			return fmt.Errorf("Prompt for input failed.\n"), 1
		}

		cmdstr, args := parseCommand(cmdstr)

		if cmdstr == "exit" {
			return handleExit(t.client, t)
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
			fmt.Printf("event error: %s\n", err)
			continue
		}

		switch event.Name {
		case api.Message:
			fmt.Printf("server=> %s\n", event.Message.Body)
		case api.BreakPointsUpdated:
			// TODO(danmace): copy
			t.cache.breakPoints = event.BreakPointsUpdated.BreakPoints
		case api.ThreadsUpdated:
			// TODO(danmace): copy
			t.cache.threads = event.ThreadsUpdated.Threads
		case api.FilesUpdated:
			t.cache.process.Files = event.FilesUpdated.Files
		default:
			fmt.Printf("unsupported event %s\n", event.Name)
		}
	}
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

	return nil, 0
}

func parseCommand(cmdstr string) (string, []string) {
	vals := strings.Split(cmdstr, " ")
	return vals[0], vals[1:]
}
