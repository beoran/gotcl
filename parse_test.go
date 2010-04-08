package gotcl

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"strings"
	"testing"
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

	s2 := FromStr("a \" b")
	_, e2 := s2.AsList()
	if e2 == nil {
		t.Fatal("Should have gotten an error.")
	}
}

func verifyParse(t *testing.T, code string) {
	_, e := ParseCommands(strings.NewReader(code))
	if e != nil {
		t.Fatalf("%v should parse, but got %#v", code, e.String())
	}
}

func TestCommandParsing(t *testing.T) {
	verifyParse(t, `set x {\{}`)
	verifyParse(t, `set x \{foo\{`)
	verifyParse(t, `set x []`)
	verifyParse(t, `set x  [  ]`)
	verifyParse(t, `set x "foo[]bar"`)
}

func testExpr(t *testing.T, vvals map[string]string, et exprtest) {
	s := et.code
	exp, e := ParseExpr(strings.NewReader(s))
	if e != nil {
		t.Errorf("%#v → %v\n", s, e)
	} else {
		i := NewInterp()
		for k, v := range vvals {
			i.SetVarRaw(k, FromStr(v))
		}
		v := exp.Eval(i)
		if i.err != nil {
			t.Errorf("Expected %s, got %v\n", et.result, i.err)
		} else if v.AsString() != et.result {
			t.Errorf("%s: Expected %s, got %#v (%s)\n", s, et.result, v.AsString(), exp.String())
		} else {
			// everything is ok
		}
	}
}

type exprtest struct {
	code, result string
}

func TestExprParse(t *testing.T) {
	cases := []exprtest{
		exprtest{"4 + 5", "9"},
		exprtest{"22", "22"},
		exprtest{"$foo", "42"},
		exprtest{"$foo - 42", "0"},
		exprtest{"44 + (4 + 5)", "53"},
		exprtest{"4 * 1 * 4 + 2 * 1 * 2", "20"},
		exprtest{"44 * 4 + 5", "181"},
		exprtest{"4 - 5 * 2 - 1", "-7"},
		exprtest{"3 - 2 - 1", "0"},
		exprtest{"1 + 2 + 3", "6"},
		exprtest{"1 + 1 * 2", "3"},
		exprtest{"(1 + 1) * 2", "4"},
		exprtest{"1 + (2 * 1 + 2)", "5"},
		exprtest{"1 + (2 + 1 * 2)", "5"},
		exprtest{"(1 + 1) * (1+1)", "4"},
		exprtest{"33 + 11 == 44", "1"},
		exprtest{"!0", "1"},
		exprtest{"!1", "0"},
		exprtest{"!1 == !0", "0"},
		exprtest{"!(1 == 0)", "1"},
		exprtest{"!(1 == 0)", "1"},
		exprtest{"[+ 1 1] == 2", "1"},
		exprtest{"1 || 0", "1"},
		exprtest{"1 && 0", "0"},
		exprtest{"1 == 1 && 0 == 0", "1"},
		exprtest{"1 || 1 && 0 || 0", "1"},
		exprtest{"1 <= 2", "1"},
		exprtest{"$foo >= 109", "0"},
		exprtest{"$foo != 42", "0"},
		exprtest{"-3 * -3", "9"},
		exprtest{"1 == 2 && 1", "0"},
	}
	varvals := map[string]string{"foo": "42"}
	for _, c := range cases {
		testExpr(t, varvals, c)
	}
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

// Loads same file as above benchmarks, but just counts
// newlines. This is to see how much of the time is spent
// actually parsing vs just reading runes.
func BenchmarkNoopParse(b *testing.B) {
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
