// Package command implements functions for responding to user
// input and dispatching to appropriate backend commands.
package terminal

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	//"strconv"
	"strings"

	api "github.com/derekparker/delve/api"
	client "github.com/derekparker/delve/client"
	"github.com/derekparker/delve/proctl"
)

type cmdfunc func(client client.Interface, cache *cache, args ...string) error

type command struct {
	aliases []string
	helpMsg string
	cmdFn   cmdfunc
}

// Returns true if the command string matches one of the aliases for this command
func (c command) match(cmdstr string) bool {
	for _, v := range c.aliases {
		if v == cmdstr {
			return true
		}
	}
	return false
}

type Commands struct {
	cmds    []command
	lastCmd cmdfunc
	cache   *cache
	client  client.Interface
}

// Returns a Commands struct with default commands defined.
func DebugCommands(cache *cache, client client.Interface) *Commands {
	c := &Commands{cache: cache, client: client}

	c.cmds = []command{
		{aliases: []string{"help"}, cmdFn: c.help, helpMsg: "Prints the help message."},
		{aliases: []string{"break", "b"}, cmdFn: breakpoint, helpMsg: "Set break point at the entry point of a function, or at a specific file/line. Example: break foo.go:13"},
		{aliases: []string{"continue", "c"}, cmdFn: cont, helpMsg: "Run until breakpoint or program termination."},
		{aliases: []string{"step", "si"}, cmdFn: step, helpMsg: "Single step through program."},
		{aliases: []string{"next", "n"}, cmdFn: next, helpMsg: "Step over to next source line."},
		{aliases: []string{"threads"}, cmdFn: threads, helpMsg: "Print out info for every traced thread."},
		{aliases: []string{"thread", "t"}, cmdFn: thread, helpMsg: "Switch to the specified thread."},
		{aliases: []string{"clear"}, cmdFn: clear, helpMsg: "Deletes breakpoint."},
		{aliases: []string{"goroutines"}, cmdFn: goroutines, helpMsg: "Print out info for every goroutine."},
		{aliases: []string{"breakpoints", "bp"}, cmdFn: breakpoints, helpMsg: "Print out info for active breakpoints."},
		{aliases: []string{"print", "p"}, cmdFn: printVar, helpMsg: "Evaluate a variable."},
		{aliases: []string{"info"}, cmdFn: info, helpMsg: "Provides info about args, funcs, locals, sources, or vars."},
		{aliases: []string{"exit"}, cmdFn: nullCommand, helpMsg: "Exit the debugger."},
	}

	return c
}

// Register custom commands. Expects cf to be a func of type cmdfunc,
// returning only an error.
func (c *Commands) Register(cmdstr string, cf cmdfunc, helpMsg string) {
	for _, v := range c.cmds {
		if v.match(cmdstr) {
			v.cmdFn = cf
			return
		}
	}

	c.cmds = append(c.cmds, command{aliases: []string{cmdstr}, cmdFn: cf, helpMsg: helpMsg})
}

// Find will look up the command function for the given command input.
// If it cannot find the command it will defualt to noCmdAvailable().
// If the command is an empty string it will replay the last command.
func (c *Commands) Find(cmdstr string) cmdfunc {
	// If <enter> use last command, if there was one.
	if cmdstr == "" {
		if c.lastCmd != nil {
			return c.lastCmd
		}
		return nullCommand
	}

	for _, v := range c.cmds {
		if v.match(cmdstr) {
			c.lastCmd = v.cmdFn
			return v.cmdFn
		}
	}

	return noCmdAvailable
}

func CommandFunc(fn func() error) cmdfunc {
	return func(client client.Interface, cache *cache, args ...string) error {
		return fn()
	}
}

func noCmdAvailable(client client.Interface, cache *cache, ars ...string) error {
	return fmt.Errorf("command not available")
}

func nullCommand(client client.Interface, cache *cache, ars ...string) error {
	return nil
}

func (c *Commands) help(client client.Interface, cache *cache, ars ...string) error {
	fmt.Println("The following commands are available:")
	for _, cmd := range c.cmds {
		fmt.Printf("\t%s - %s\n", strings.Join(cmd.aliases, "|"), cmd.helpMsg)
	}
	return nil
}

func threads(client client.Interface, cache *cache, ars ...string) error {
	/*
		for _, th := range cache.threads {
			prefix := "  "
			if th.IsCurrent {
				prefix = "* "
			}
			if th.CurrentLine != nil {
				fmt.Printf("%sThread %d at %#v %s:%d %s\n",
					prefix, th.ID, th.CurrentPC, th.CurrentLine.File,
					th.CurrentLine.Line, th.CurrentLine.Name)
			} else {
				fmt.Printf("%sThread %d at %#v\n", prefix, th.ID, th.CurrentPC)
			}
		}
	*/
	return nil
}

func thread(client client.Interface, cache *cache, ars ...string) error {
	/*
		oldTid := p.CurrentThread.Id
		tid, err := strconv.Atoi(ars[0])
		if err != nil {
			return err
		}

		err = p.SwitchThread(tid)
		if err != nil {
			return err
		}

		fmt.Printf("Switched from %d to %d\n", oldTid, tid)
	*/
	return nil
}

func goroutines(client client.Interface, cache *cache, ars ...string) error {
	return nil
	//return p.PrintGoroutinesInfo()
}

func cont(client client.Interface, cache *cache, ars ...string) error {
	/*
		err := p.Continue()
		if err != nil {
			return err
		}

		return printcontext(p)
	*/
	return nil
}

func step(client client.Interface, cache *cache, args ...string) error {
	/*
		err := p.Step()
		if err != nil {
			return err
		}

		return printcontext(p)
	*/
	return nil
}

func next(client client.Interface, cache *cache, args ...string) error {
	/*
		err := p.Next()
		if err != nil {
			return err
		}

		return printcontext(p)
	*/
	return nil
}

func clear(client client.Interface, cache *cache, args ...string) error {
	/*
		if len(args) == 0 {
			return fmt.Errorf("not enough arguments")
		}

		bp, err := p.ClearByLocation(args[0])
		if err != nil {
			return err
		}

		fmt.Printf("Breakpoint %d cleared at %#v for %s %s:%d\n", bp.ID, bp.Addr, bp.FunctionName, bp.File, bp.Line)
	*/
	return nil
}

type ById []*api.BreakPoint

func (a ById) Len() int           { return len(a) }
func (a ById) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ById) Less(i, j int) bool { return a[i].ID < a[j].ID }

func breakpoints(client client.Interface, cache *cache, args ...string) error {
	// TODO: don't mutate the input
	sort.Sort(ById(cache.breakPoints))
	for _, bp := range cache.breakPoints {
		fmt.Println(bp)
	}

	return nil
}

func breakpoint(client client.Interface, cache *cache, args ...string) error {
	if len(args) == 0 {
		return fmt.Errorf("not enough arguments")
	}

	location := args[0]

	err := client.AddBreakPoint(location)
	if err != nil {
		return err
	}

	fmt.Printf("Breakpoint set at %s\n", location)

	return nil
}

func printVar(client client.Interface, cache *cache, args ...string) error {
	/*
		if len(args) == 0 {
			return fmt.Errorf("not enough arguments")
		}

		val, err := p.EvalSymbol(args[0])
		if err != nil {
			return err
		}

		fmt.Println(val.Value)
	*/
	return nil
}

func filterVariables(vars []*proctl.Variable, filter *regexp.Regexp) []string {
	/*
		data := make([]string, 0, len(vars))
		for _, v := range vars {
			if v == nil {
				continue
			}
			if filter == nil || filter.Match([]byte(v.Name)) {
				data = append(data, fmt.Sprintf("%s = %s", v.Name, v.Value))
			}
		}
		return data
	*/
	return nil
}

func info(client client.Interface, cache *cache, args ...string) error {
	/*
		if len(args) == 0 {
			return fmt.Errorf("not enough arguments. expected info type [regex].")
		}

		// Allow for optional regex
		var filter *regexp.Regexp
		if len(args) >= 2 {
			var err error
			if filter, err = regexp.Compile(args[1]); err != nil {
				return fmt.Errorf("invalid filter argument: %s", err.Error())
			}
		}

		var data []string

		switch args[0] {
		case "sources":
			data = make([]string, 0, len(p.GoSymTable.Files))
			for f := range p.GoSymTable.Files {
				if filter == nil || filter.Match([]byte(f)) {
					data = append(data, f)
				}
			}

		case "funcs":
			data = make([]string, 0, len(p.GoSymTable.Funcs))
			for _, f := range p.GoSymTable.Funcs {
				if f.Sym != nil && (filter == nil || filter.Match([]byte(f.Name))) {
					data = append(data, f.Name)
				}
			}

		case "args":
			vars, err := p.CurrentThread.FunctionArguments()
			if err != nil {
				return nil
			}
			data = filterVariables(vars, filter)

		case "locals":
			vars, err := p.CurrentThread.LocalVariables()
			if err != nil {
				return nil
			}
			data = filterVariables(vars, filter)

		case "vars":
			vars, err := p.CurrentThread.PackageVariables()
			if err != nil {
				return nil
			}
			data = filterVariables(vars, filter)

		default:
			return fmt.Errorf("unsupported info type, must be args, funcs, locals, sources, or vars")
		}

		// sort and output data
		sort.Sort(sort.StringSlice(data))

		for _, d := range data {
			fmt.Println(d)
		}
	*/
	return nil
}

func printcontext(p *proctl.DebuggedProcess) error {
	var context []string

	regs, err := p.Registers()
	if err != nil {
		return err
	}

	f, l, fn := p.GoSymTable.PCToLine(regs.PC())

	if fn != nil {
		fmt.Printf("current loc: %s %s:%d\n", fn.Name, f, l)
		file, err := os.Open(f)
		if err != nil {
			return err
		}
		defer file.Close()

		buf := bufio.NewReader(file)
		for i := 1; i < l-5; i++ {
			_, err := buf.ReadString('\n')
			if err != nil && err != io.EOF {
				return err
			}
		}

		for i := l - 5; i <= l+5; i++ {
			line, err := buf.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					return err
				}

				if err == io.EOF {
					break
				}
			}

			arrow := "  "
			if i == l {
				arrow = "=>"
			}

			context = append(context, fmt.Sprintf("\033[34m%s %d\033[0m: %s", arrow, i, line))
		}
	} else {
		fmt.Printf("Stopped at: 0x%x\n", regs.PC())
		context = append(context, "\033[34m=>\033[0m    no source available")
	}

	fmt.Println(strings.Join(context, ""))

	return nil
}
