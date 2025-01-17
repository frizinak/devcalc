package flags

import (
	"flag"
	"fmt"
	"io"
	"os"
)

type Set struct {
	w        io.Writer
	f        *flag.FlagSet
	name     string
	children map[string]*Set
	handler  Handler
}

func New(f *flag.FlagSet, output io.Writer) *Set {
	f.SetOutput(output)
	return &Set{w: output, f: f, name: f.Name(), children: make(map[string]*Set)}
}

func NewRoot(output io.Writer) *Set {
	return New(flag.CommandLine, output)
}

func (f *Set) Name() string { return f.name }

func (f *Set) Define(cb func(*flag.FlagSet) func(io.Writer)) *Set {
	helper := cb(f.f)
	f.f.Usage = func() {
		if helper == nil {
			fmt.Fprintln(f.w, "[flags]")
			f.f.PrintDefaults()
			return
		}
		helper(f.w)
	}

	return f
}

func (f *Set) Handler(h Handler) *Set { f.handler = h; return f }

func (f *Set) Add(name string) *Set {
	if n, ok := f.children[name]; ok {
		return n
	}

	fname := f.name + " " + name
	rf := flag.NewFlagSet(fname, flag.ExitOnError)
	rf.SetOutput(f.w)
	n := New(rf, f.w)
	f.children[name] = n

	return n
}

func (f *Set) Usage(ex int) {
	f.f.Usage()
	os.Exit(ex)
}

func (f *Set) Args() []string { return f.f.Args() }

func (f *Set) ParseCommandline() (sub *Set, trail []string) {
	return f.Parse(os.Args[1:])
}

func (f *Set) Parse(args []string) (sub *Set, trail []string) {
	sub, trail = f.parse(args, make([]string, 0))
	return sub, trail
}

func (f *Set) parse(args, trail []string) (*Set, []string) {
	f.f.Parse(args)
	cmds := f.f.Args()
	if len(cmds) == 0 {
		if f.handler == nil {
			f.Usage(1)
		}

		return f, trail
	}

	if sub, ok := f.children[cmds[0]]; ok {
		return sub.parse(cmds[1:], append(trail, cmds[0]))
	}

	if f.handler == nil {
		f.Usage(1)
	}

	return f, trail
}

func (f *Set) Do() error {
	return f.handler(f, f.Args())
}

type Handler func(f *Set, args []string) error
