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
	"os"
	"strings"
)

// Cmd represents a sub command, allowing to define subcommand
// flags and runnable to run once arguments match the subcommand
// requirements.
type Cmd interface {
	Flags(*flag.FlagSet) *flag.FlagSet
	Run(args []string)
}

type Commands struct {
	// the name of program
	program string

	// the flags of global
	flags *flag.FlagSet

	// A map of all of the registered sub-commands.
	cmds map[string]*cmdInstance

	// Matching subcommand.
	matchingCmd *cmdInstance

	// Arguments to call subcommand's runnable.
	args []string

	// Flag to determine whether help is
	// asked for subcommand or not
	flagHelp bool
}

func New(program string, flags *flag.FlagSet) *Commands {
	return &Commands{program: program, flags: flags, cmds: map[string]*cmdInstance{}}
}

type cmdInstance struct {
	name          string
	desc          string
	command       Cmd
	requiredFlags []string
}

// Registers a Cmd for the provided sub-command name. E.g. name is the
// `status` in `git status`.
func (self *Commands) On(name, description string, command Cmd, requiredFlags []string) {
	self.cmds[name] = &cmdInstance{
		name:          name,
		desc:          description,
		command:       command,
		requiredFlags: requiredFlags,
	}
}

// Prints the usage.
func (self *Commands) Usage() {
	if len(self.cmds) == 0 {
		// no subcommands
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", self.program)
		self.flags.PrintDefaults()
		return
	}

	fmt.Fprintf(os.Stderr, "Usage: %s <command>\n\n", self.program)
	fmt.Fprintf(os.Stderr, "where <command> is one of:\n")
	for name, subcmd := range self.cmds {
		fmt.Fprintf(os.Stderr, "  %-15s %s\n", name, subcmd.desc)
	}

	// Returns the total number of globally registered flags.
	count := 0
	self.flags.VisitAll(func(flag *flag.Flag) {
		count++
	})

	if count > 0 {
		fmt.Fprintf(os.Stderr, "\navailable flags:\n")
		self.flags.PrintDefaults()
	}
	fmt.Fprintf(os.Stderr, "\n%s <command> -h for subcommand help\n", self.program)
}

func (self *Commands) subcommandUsage(subcmd *cmdInstance) {
	fmt.Fprintf(os.Stderr, "%s\r\n", subcmd.desc)
	// should only output sub command flags, ignore h flag.
	fs := subcmd.command.Flags(flag.NewFlagSet(subcmd.name, flag.ContinueOnError))
	flagCount := 0
	fs.VisitAll(func(flag *flag.Flag) { flagCount++ })
	if flagCount > 0 {
		fmt.Fprintf(os.Stderr, "Usage of %s %s:\n", self.program, subcmd.name)
		fs.PrintDefaults()
		if len(subcmd.requiredFlags) > 0 {
			fmt.Fprintf(os.Stderr, "\nrequired flags:\n")
			fmt.Fprintf(os.Stderr, "  %s\n\n", strings.Join(subcmd.requiredFlags, ", "))
		}
	}
}

// Parses the flags and leftover arguments to match them with a
// sub-command. Evaluate all of the global flags and register
// sub-command handlers before calling it. Sub-command handler's
// `Run` will be called if there is a match.
// A usage with flag defaults will be printed if provided arguments
// don't match the configuration.
// Global flags are accessible once Parse executes.
func (self *Commands) Parse(args []string) {
	// if there are no subcommands registered,
	// return immediately
	if len(self.cmds) < 1 {
		return
	}

	if len(args) < 1 {
		self.Usage()
		os.Exit(1)
	}

	name := args[0]
	subcmd, ok := self.cmds[name]
	if !ok {
		self.Usage()
		os.Exit(1)
	}

	fs := subcmd.command.Flags(flag.NewFlagSet(name, flag.ExitOnError))
	// fs.BoolVar(&self.flagHelp, "h", false, "")
	// fs.BoolVar(&self.flagHelp, "?", false, "")
	// fs.BoolVar(&self.flagHelp, "help", false, "")
	// fs.BoolVar(&self.flagHelp, "-help", false, "")

	self.matchingCmd = subcmd
	fs.Usage = func() {
		self.subcommandUsage(subcmd)
	}
	fs.Parse(args[1:])
	self.args = fs.Args()

	// Check for required flags.
	flagMap := make(map[string]bool)
	for _, flagName := range subcmd.requiredFlags {
		flagMap[flagName] = true
	}
	fs.Visit(func(f *flag.Flag) {
		delete(flagMap, f.Name)
	})
	if len(flagMap) > 0 {
		self.subcommandUsage(self.matchingCmd)
		os.Exit(1)
	}
}

// Runs the subcommand's runnable. If there is no subcommand
// registered, it silently returns.
func (self *Commands) Run() {
	if self.matchingCmd != nil {
		if self.flagHelp {
			self.subcommandUsage(self.matchingCmd)
			return
		}
		self.matchingCmd.command.Run(self.args)
	}
}

// Parses flags and run's matching subcommand's runnable.
func (self *Commands) ParseAndRun(args []string) {
	self.Parse(args)
	self.Run()
}

var Default = New(os.Args[0], flag.CommandLine)

func On(name, description string, command Cmd, requiredFlags []string) {
	Default.On(name, description, command, requiredFlags)
}

func Usage() {
	Default.Usage()
}

func Parse() {
	flag.Usage = Default.Usage
	flag.Parse()
	Default.Parse(flag.Args())
}

func Run() {
	Default.Run()
}

func ParseAndRun() {
	Parse()
	Run()
}
