package main

import (
	"bufio"
	"flag"
	"fmt"
	"gotcl"
	"io"
	"os"
	"runtime"
)

var nogc = flag.Bool("nogc", false, "if true, gc is disabled")

func RunRepl(in io.Reader, out io.Writer, fn func(string) (string, error)) {
	inbuf := bufio.NewReader(in)
	for {
		fmt.Fprint(out, "> ")
		ln, err := inbuf.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				fmt.Fprintln(out, err.Error())
			}
			break
		}
		if len(ln) == 0 {
			continue
		}
		res, rerr := fn(ln)
		if rerr != nil {
			fmt.Fprintln(out, "Error: "+rerr.Error())
		} else {
			if len(res) != 0 {
				fmt.Fprintln(out, res)
			}
		}
	}
}

func RunTclRepl(in io.Reader, out io.Writer) {
	i := gotcl.NewInterp()
	setArgs(i, flag.Args(), true)
	RunRepl(in, out, func(ln string) (string, error) {
		res, e := i.EvalString(ln)
		i.ClearError()
		if e != nil {
			return "", e
		}
		return res.AsString(), e
	})
}

func setArgs(i *gotcl.Interp, args []string, interactive bool) {
	i.SetVarRaw("argc", gotcl.FromInt(len(args)-1))
	if len(args) > 0 {
		i.SetVarRaw("argv0", gotcl.FromStr(args[0]))
		i.SetVarRaw("argv", gotcl.FromList(args[1:]))
	} else {
		i.SetVarRaw("argv", gotcl.FromList(args))
	}
	i.SetVarRaw("tcl_interactive", gotcl.FromBool(interactive))
}

func main() {
	flag.Parse()
	if *nogc {
		runtime.MemStats.EnableGC = false
		println("GC disabled.")
	}
	args := flag.Args()
	if len(args) == 1 {
		filename := args[0]
		file, e := os.Open(filename)
		if e != nil {
			panic(e.Error())
		}
		defer file.Close()
		i := gotcl.NewInterp()
		setArgs(i, args, false)
		_, err := i.Run(file)
		if err != nil {
			fmt.Println("Error: " + err.Error())
		}
	} else {
		RunTclRepl(os.Stdin, os.Stdout)
	}
}
