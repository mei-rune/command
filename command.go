// Copyright 2013 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package command allows you to define subcommands
// for your command line interfaces. It extends the flag package
// to provide flag support for subcommands.
package command

import (
	"flag"
	"fmt"
	"io"
	"os"
	"errors"
)

var StdOutput io.Writer = os.Stdout
var StdErr io.Writer = os.Stderr

func Println(args ...interface{}) {
	fmt.Fprintln(StdOutput, args...)
}

func Printf(msg string, args ...interface{}) {
	fmt.Fprintf(StdOutput, msg, args...)
}

func ErrOutput(msg string, args ...interface{}) {
	fmt.Fprintf(StdErr, msg, args...)
	fmt.Fprintln(StdErr, "")
}

// Cmd represents a sub command, allowing to define subcommand
// flags and runnable to run once arguments match the subcommand
// requirements.
type Cmd interface {
	Flags(*flag.FlagSet) *flag.FlagSet
	Run(args []string) error
}

type Commands struct {
	// the name of program
	program string

	// the flags of global
	flags *flag.FlagSet

	// A map of all of the registered sub-commands.
	list []*cmdInstance

	// Matching subcommand.
	matchingCmd *cmdInstance

	// Arguments to call subcommand's runnable.
	args []string

	// Flag to determine whether help is
	// asked for subcommand or not
	flagHelp bool
}

func New(program string, flags *flag.FlagSet) *Commands {
	return &Commands{program: program, flags: flags}
}

type cmdInstance struct {
	name          string
	description   string
	command       Cmd
	requiredFlags []string
}

// Registers a Cmd for the provided sub-command name. E.g. name is the
// `status` in `git status`.
func (c *Commands) On(name, description string, command Cmd, requiredFlags []string) {
	for _, subcmd := range c.list {
		if subcmd.name == name {
			panic(errors.New("命令 '"+name+"' 已存在"))
		}
	}
	c.list = append(c.list, &cmdInstance{
		name:          name,
		description:   description,
		command:       command,
		requiredFlags: requiredFlags,
	})
}

// Prints the usage.
func (c *Commands) Usage() {
	if len(c.list) == 0 {
		// no subcommands
		fmt.Fprintf(StdErr, "使用方法: %s [选项]\n", c.program)
		c.flags.PrintDefaults()
		return
	}

	fmt.Fprintf(StdErr, "使用方法: %s [选项] 子命令 [选项] \n\n", c.program)
	fmt.Fprintf(StdErr, "子命令列表:\n")
	for _, subcmd := range c.list {
		fmt.Fprintf(StdErr, "  %-15s %s\n", subcmd.name, subcmd.description)
	}

	// Returns the total number of globally registered flags.
	count := 0
	c.flags.VisitAll(func(flag *flag.Flag) {
		count++
	})

	if count > 0 {
		fmt.Fprintf(StdErr, "\n选项:\n")
		c.flags.PrintDefaults()
	}
	fmt.Fprintf(StdErr, "\n查看子命令的帮助: %s 子命令 -h\n", c.program)
}

func (c *Commands) SubcommandUsage(subcmd *cmdInstance) {
	fmt.Fprintf(StdErr, "%s\r\n", subcmd.description)
	// should only output sub command flags, ignore h flag.
	fs := subcmd.command.Flags(flag.NewFlagSet(subcmd.name, flag.ContinueOnError))
	flagCount := 0
	fs.VisitAll(func(flag *flag.Flag) { flagCount++ })
	if flagCount > 0 {
		fmt.Fprintf(StdErr, "使用方法: %s %s [选项]\n", c.program, subcmd.name)
		fs.PrintDefaults()
	}
}

// Parses the flags and leftover arguments to match them with a
// sub-command. Evaluate all of the global flags and register
// sub-command handlers before calling it. Sub-command handler's
// `Run` will be called if there is a match.
// A usage with flag defaults will be printed if provided arguments
// don't match the configuration.
// Global flags are accessible once Parse executes.
func (c *Commands) Parse(args []string) {
	// if there are no subcommands registered,
	// return immediately
	if len(c.list) < 1 {
		return
	}

	if len(args) < 1 {
		c.Usage()
		os.Exit(1)
		return
	}
	
	name := args[0]
	var subcmd *cmdInstance
	for _, sub := range c.list {
		if sub.name == name {
			subcmd = sub
			break
		}
	}
	if subcmd == nil {
		c.Usage()
		os.Exit(1)
	}

	fs := flag.NewFlagSet(name, flag.ExitOnError)
	fs = subcmd.command.Flags(fs)
	fs.BoolVar(&c.flagHelp, "h", false, "")
	fs.BoolVar(&c.flagHelp, "?", false, "")
	fs.BoolVar(&c.flagHelp, "help", false, "")
	// fs.BoolVar(&c.flagHelp, "-help", false, "")

	c.matchingCmd = subcmd
	fs.Usage = func() {
		c.SubcommandUsage(subcmd)
	}
	fs.Parse(args[1:])
	c.args = fs.Args()

	// Check for required flags.
	flagMap := make(map[string]bool)
	for _, flagName := range subcmd.requiredFlags {
		flagMap[flagName] = true
	}
	fs.Visit(func(f *flag.Flag) {
		delete(flagMap, f.Name)
	})
	if len(flagMap) > 0 {
		c.SubcommandUsage(c.matchingCmd)
		os.Exit(1)
	}
}

// Runs the subcommand's runnable. If there is no subcommand
// registered, it silently returns.
func (c *Commands) Run() {
	if c.matchingCmd != nil {
		if c.flagHelp {
			c.SubcommandUsage(c.matchingCmd)
			return
		}

		if err := c.matchingCmd.command.Run(c.args); err != nil {
			var code = -1
			var help = false
			if e, ok := err.(*Error); ok {
				code = e.Code
				help = e.Help
			}

			ErrOutput("FATAL: %s", err.Error())
			if help {
				c.SubcommandUsage(c.matchingCmd)
			}
			os.Exit(code)
			return
		}
	}
}

// Parses flags and run's matching subcommand's runnable.
func (c *Commands) ParseAndRun(args []string) {
	c.Parse(args)
	c.Run()
}

var Default = New(os.Args[0], flag.CommandLine)

func On(name, description string, command Cmd, requiredFlags []string) {
	Default.On(name, description, command, requiredFlags)
}

func Usage() {
	Default.Usage()
}

var DefaultCommandName string


var defaultParsePostHook func()

func SetDefaultParsePostHook(hook func()) {
	defaultParsePostHook = hook
}

func Parse() {
	flag.Usage = Default.Usage
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		args = []string{DefaultCommandName}
	}
	if defaultParsePostHook != nil {
		defaultParsePostHook()
	}
	Default.Parse(args)
}

func Run() {
	Default.Run()
}

func ParseAndRun() {
	Parse()
	Run()
}

type Error struct {
	Code    int
	Message string
	Help    bool
}

func (e *Error) Error() string {
	return e.Message
}
