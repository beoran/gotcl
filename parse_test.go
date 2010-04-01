package gotcl

import (
	"strings"
	"testing"
	"bufio"
	"bytes"
	"io/ioutil"
)


func TestListParse(t *testing.T) {
	s := FromStr("{x}")
	ll, e := s.AsList()
	if e != nil {
		t.Fatal(e.String())
	}
	if len(ll) != 1 {
		t.Fatalf("len({x}) should be 1, was %#v", ll)
	}
}

func verifyParse(t *testing.T, code string) {
	_, e := ParseCommands(strings.NewReader(code))
	if e != nil {
		t.Fatalf("%#v should parse, but got %#v", code, e.String())
	}
}

func TestCommandParsing(t *testing.T) {
	verifyParse(t, `set x {\{}`)
	verifyParse(t, `set x \{foo\{`)
	verifyParse(t, `set x []`)
	verifyParse(t, `set x  [  ]`)
	verifyParse(t, `set x "foo[]bar"`)
}

func BenchmarkParsing(b *testing.B) {
	b.StopTimer()
	data, err := ioutil.ReadFile("parsebench.tcl")
	if err != nil {
		panic(err.String())
	}
	b.SetBytes(int64(len(data)))
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		reader := bytes.NewBuffer(data)
		_, e := ParseCommands(reader)
		if e != nil {
			panic(e.String())
		}
	}
}

func BenchmarkListParsing(b *testing.B) {
	b.StopTimer()
	data, err := ioutil.ReadFile("parsebench.tcl")
	if err != nil {
		panic(err.String())
	}
	b.SetBytes(int64(len(data)))
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		reader := bytes.NewBuffer(data)
		_, e := ParseList(reader)
		if e != nil {
			panic(e.String())
		}
	}
}

func BenchmarkDoNothing(b *testing.B) {
	b.StopTimer()
	data, err := ioutil.ReadFile("parsebench.tcl")
	if err != nil {
		panic(err.String())
	}
	b.SetBytes(int64(len(data)))
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		nlcount := 0
		reader := bufio.NewReader(bytes.NewBuffer(data))
		done := false
		for !done {
			r, _, e := reader.ReadRune()
			if e != nil {
				done = true
			}
			if r == '\n' {
				nlcount++
			}
		}
	}
}

func eat(i *Interp, args []*TclObj) TclStatus { return kTclOK }

func run_t(text string, t *testing.T) {
	i := NewInterp()
	i.SetCmd("eat", eat)
	_, err := i.Run(strings.NewReader(text))
	if err != nil {
		t.Fatal(err)
	}
}

func TestParse(t *testing.T) {
	run := func(s string) { run_t(s, t) }
	run(`eat [concat "Hi" " Mom!"]`)

	run(`
set x 95
eat "Number: $x yay "
# puts "4 plus 5 is [+ fish 10]!"
eat "10 plus 10 is [+ 10 10]!"
`)
	run(`
eval { eat "Hello!" }
if {+ 1 0} {
    eat "Yep"
}
eat "Length: [llength {1 2 3 4 5}]"
set i 10
eat $i
set i [+ $i 1]
eat $i`)
	run(`
proc say_hello {} {
    return 5
    error "This should not be printed!"
}
set v [say_hello]
eat "5 == $v"
proc double {} {
    say_hello
    eat "This should be seen."
}`)
	run(`
set {a b c} 44
eat ${a b c}
eat "It is ${a b c}."
    `)
}
