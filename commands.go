package gotcl

import (
	"fmt"
	"io"
	"time"
	"os"
	"bytes"
	"bufio"
	"utf8"
	"strings"
)

func tclSet(i *Interp, args []*TclObj) TclStatus {
	if len(args) == 0 || len(args) > 2 {
		return i.FailStr("wrong # args")
	}
	if len(args) == 2 {
		val := args[1]
		i.SetVar(args[0].asVarRef(), val)
		return i.Return(val)
	}
	v, e := i.GetVar(args[0].asVarRef())
	if e != nil {
		return i.Fail(e)
	}
	return i.Return(v)
}

func tclUnset(i *Interp, args []*TclObj) TclStatus {
	i.SetVarRaw(args[0].AsString(), nil)
	return kTclOK
}

func tclUplevel(i *Interp, args []*TclObj) TclStatus {
	if len(args) != 1 {
		return i.FailStr("wrong # args")
	}
	orig_frame := i.frame
	i.frame = i.frame.up()
	rc := i.EvalObj(args[0])
	i.frame = orig_frame
	return rc
}

func tclUpvar(i *Interp, args []*TclObj) TclStatus {
	if len(args) != 2 && len(args) != 3 {
		return i.FailStr("wrong # args")
	}
	level := 1
	if len(args) == 3 {
		ll, e := args[0].AsInt()
		if e != nil {
			return i.Fail(e)
		}
		level = ll
		args = args[1:]
	}
	oldn := args[0].AsString()
	newn := args[1].AsString()
	i.LinkVar(level, oldn, newn)
	return i.Return(kNil)
}

func tclIncr(i *Interp, args []*TclObj) TclStatus {
	if len(args) != 1 && len(args) != 2 {
		return i.FailStr("wrong # args")
	}
	vn := args[0].asVarRef()
	v, ve := i.GetVar(vn)
	if ve != nil {
		return i.Fail(ve)
	}

	inc := 1
	if len(args) == 2 {
		incv, ie := args[1].AsInt()
		if ie != nil {
			return i.Fail(ie)
		}
		inc = incv
	}
	iv, err := v.AsInt()
	if err != nil {
		return i.Fail(err)
	}
	res := FromInt(iv + inc)
	i.SetVar(vn, res)
	return i.Return(res)
}

func tclReturn(i *Interp, args []*TclObj) TclStatus {
	if len(args) == 0 {
		i.retval = kNil
		return kTclReturn
	} else if len(args) == 1 {
		i.retval = args[0]
		return kTclReturn
	}
	return i.FailStr("wrong # args")
}

func tclBreak(i *Interp, args []*TclObj) TclStatus {
	if len(args) != 0 {
		return i.FailStr("wrong # args")
	}
	return kTclBreak
}

func tclContinue(i *Interp, args []*TclObj) TclStatus {
	if len(args) != 0 {
		return i.FailStr("wrong # args")
	}
	return kTclContinue
}

func tclCatch(i *Interp, args []*TclObj) TclStatus {
	if len(args) == 0 {
		return i.FailStr("wrong # args to catch")
	}
	r := i.EvalObj(args[0])
	if len(args) == 2 && r == kTclErr {
		i.SetVarRaw(args[1].AsString(), fromStr(i.err.String()))
	}
	i.ClearError()
	return i.Return(FromInt(int(r)))
}

func tclIf(i *Interp, args []*TclObj) TclStatus {
	if len(args) < 2 {
		return i.FailStr("wrong # args")
	}
	cond, err := args[0].asExpr()
	if err != nil {
		return i.Fail(err)
	}
	args = args[1:]
	if args[0].AsString() == "then" {
		args = args[1:]
	}
	body := args[0]
	args = args[1:]
	var elseblock *TclObj
	if len(args) > 0 {
		if args[0].AsString() == "else" {
			if len(args) == 1 {
				return i.FailStr("wrong # args: no script following 'else' argument")
			}
			args = args[1:]
		}
		if len(args) > 0 {
			elseblock = args[0]
		}
	}
	rc1 := cond.Eval(i)
	if rc1 != kTclOK {
		return rc1
	}

	if i.retval.AsBool() {
		return i.EvalObj(body)
	} else if elseblock != nil {
		return i.EvalObj(elseblock)
	}
	return i.Return(kNil)
}

func tclExit(i *Interp, args []*TclObj) TclStatus {
	code := 0
	if len(args) == 1 {
		iv, err := args[0].AsInt()
		if err != nil {
			return i.Fail(err)
		}
		code = iv
	} else if len(args) != 0 {
		i.FailStr("wrong # args")
	}
	os.Exit(code)
	return kTclOK
}

func tclWhile(i *Interp, args []*TclObj) TclStatus {
	if len(args) != 2 {
		return i.FailStr("wrong # args")
	}
	test, body := args[0], args[1]
	testexpr, terr := test.asExpr()
	if terr != nil {
		return i.Fail(terr)
	}
	rc := testexpr.Eval(i)
	if rc != kTclOK {
		return rc
	}
	cond := i.retval.AsBool()
	for cond {
		rc = i.EvalObj(body)
		if rc == kTclBreak {
			break
		} else if rc != kTclOK && rc != kTclContinue {
			return rc
		}
		rc = testexpr.Eval(i)
		if rc != kTclOK {
			return rc
		}
		cond = i.retval.AsBool()
	}
	return i.Return(kNil)
}

func tclFor(i *Interp, args []*TclObj) TclStatus {
	if len(args) != 4 {
		return i.FailStr("wrong # args: should be \"for start test next command\"")
	}
	start, test, next, body := args[0], args[1], args[2], args[3]
	testexpr, terr := test.asExpr()
	if terr != nil {
		return i.Fail(terr)
	}
	rc := i.EvalObj(start)
	if rc != kTclOK {
		return rc
	}
	rc = testexpr.Eval(i)
	if rc != kTclOK {
		return rc
	}

	cond := i.retval.AsBool()
	for cond {
		rc = i.EvalObj(body)
		if rc == kTclBreak {
			break
		} else if rc != kTclOK && rc != kTclContinue {
			return rc
		}
		i.EvalObj(next)
		testexpr.Eval(i)
		cond = i.retval.AsBool()
	}
	return i.Return(kNil)
}

func tclForeach(i *Interp, args []*TclObj) TclStatus {
	if len(args) != 3 {
		return i.FailStr("wrong # args: should be \"foreach varName list body\"")
	}
	vname := args[0].AsString()
	list, err := args[1].AsList()
	if err != nil {
		return i.Fail(err)
	}
	body := args[2]
	for _, v := range list {
		i.SetVarRaw(vname, v)
		rc := i.EvalObj(body)
		if rc == kTclBreak {
			break
		} else if rc != kTclOK && rc != kTclContinue {
			return rc
		}
	}
	return i.Return(kNil)
}

func asInts(a *TclObj, b *TclObj) (ai int, bi int, e os.Error) {
	bi, e = b.AsInt()
	ai, e = a.AsInt()
	return
}


func tclNot(i *Interp, args []*TclObj) TclStatus {
	if len(args) != 1 {
		return i.FailStr("wrong # args")
	}
	return i.Return(FromBool(!args[0].AsBool()))
}

func cmpcmd(fn func(*TclObj, *TclObj) bool) TclCmd {
	return func(i *Interp, args []*TclObj) TclStatus {
		if len(args) != 2 {
			return i.FailStr("wrong # args")
		}
		return i.Return(FromBool(fn(args[0], args[1])))
	}
}

func intcmd(fn func(int, int) int) TclCmd {
	return func(i *Interp, args []*TclObj) TclStatus {
		a, b, e := asInts(args[0], args[1])
		if e != nil {
			return i.Fail(e)
		}
		return i.Return(FromInt(fn(a, b)))
	}
}
func intcmpcmd(fn func(int, int) bool) TclCmd {
	return func(i *Interp, args []*TclObj) TclStatus {
		if len(args) != 2 {
			return i.FailStr("wrong # args")
		}
		a, b, e := asInts(args[0], args[1])
		if e != nil {
			return i.Fail(e)
		}
		return i.Return(FromBool(fn(a, b)))
	}
}


func init() {
	plus := intcmd(func(a, b int) int { return a + b })
	minus := intcmd(func(a, b int) int { return a - b })
	times := intcmd(func(a, b int) int { return a * b })
	tclOr := cmpcmd(func(a, b *TclObj) bool { return a.AsBool() || b.AsBool() })
	tclAnd := cmpcmd(func(a, b *TclObj) bool { return a.AsBool() && b.AsBool() })
	equalTo := cmpcmd(func(a, b *TclObj) bool { return a.AsString() == b.AsString() })
	notEqualTo := cmpcmd(func(a, b *TclObj) bool { return a.AsString() != b.AsString() })
	lessThan := intcmpcmd(func(a, b int) bool { return a < b })
	lessThanEq := intcmpcmd(func(a, b int) bool { return a <= b })
	greaterThan := intcmpcmd(func(a, b int) bool { return a > b })
	greaterThanEq := intcmpcmd(func(a, b int) bool { return a >= b })
	initCmds := map[string]TclCmd{
		"set":      tclSet,
		"if":       tclIf,
		"eval":     tclEval,
		"info":     tclInfo,
		"catch":    tclCatch,
		"while":    tclWhile,
		"for":      tclFor,
		"foreach":  tclForeach,
		"uplevel":  tclUplevel,
		"return":   tclReturn,
		"break":    tclBreak,
		"continue": tclContinue,
		"upvar":    tclUpvar,
		"incr":     tclIncr,
		"exit":     tclExit,
		"+":        plus,
		"-":        minus,
		"*":        times,
		"<":        lessThan,
		">":        greaterThan,
		">=":       greaterThanEq,
		"<=":       lessThanEq,
		"==":       equalTo,
		"!=":       notEqualTo,
		"||":       tclOr,
		"&&":       tclAnd,
		"!":        tclNot,
		"unset":    tclUnset,
		"list":     tclList,
		"llength":  tclLlength,
		"lindex":   tclLindex,
		"lappend":  tclLappend,
		"lsearch":  tclLsearch,
		"concat":   tclConcat,
		"gets":     tclGets,
		"flush":    tclFlush,
		"time":     tclTime,
		"puts":     tclPuts,
		"string":   tclString,
		"split":    tclSplit,
		"source":   tclSource,
		"apply":    tclApply,
		"rename":   tclRename,
		"expr":     tclExpr,
	}
	for k, v := range initCmds {
		tclBasicCmds[k] = v
	}
}


func tclLlength(i *Interp, args []*TclObj) TclStatus {
	if len(args) != 1 {
		return i.FailStr("wrong # args")
	}
	l, err := args[0].AsList()
	if err != nil {
		return i.Fail(err)
	}
	return i.Return(FromInt(len(l)))
}

func tclList(i *Interp, args []*TclObj) TclStatus {
	return i.Return(fromList(args))
}

func tclLindex(i *Interp, args []*TclObj) TclStatus {
	l, err := args[0].AsList()
	if err != nil {
		return i.Fail(err)
	}
	ind, err := args[1].AsInt()
	if err != nil {
		i.Fail(err)
	}
	if ind >= len(l) {
		i.FailStr("out of bounds")
	}
	return i.Return(l[ind])
}

func concat(args []*TclObj) *TclObj {
	result := bytes.NewBufferString("")
	for ind, x := range args {
		if ind != 0 {
			result.WriteString(" ")
		}
		result.WriteString(strings.TrimSpace(x.AsString()))
	}
	return fromStr(result.String())
}

func tclEval(i *Interp, args []*TclObj) TclStatus {
	if len(args) == 1 {
		return i.EvalObj(args[0])
	}
	return i.EvalObj(concat(args))
}

func tclConcat(i *Interp, args []*TclObj) TclStatus {
	return i.Return(concat(args))
}

func tclLappend(i *Interp, args []*TclObj) TclStatus {
	if len(args) == 0 {
		return i.FailStr("wrong # args")
	}
	vname := args[0].AsString()
	v, ve := i.GetVarRaw(vname)
	if ve != nil {
		i.ClearError()
		v = fromList(make([]*TclObj, 0, 10))
	}
	items, err := v.AsList()
	if err != nil {
		return i.Fail(err)
	}
	new_items := args[1:]
	dest := items
	new_len := len(items) + len(new_items)
	if cap(dest) < new_len {
		dest = make([]*TclObj, new_len, 2*new_len+4)
		copy(dest, items)
	}
	dest = dest[0:new_len]
	copy(dest[len(items):], new_items)
	newobj := fromList(dest)
	i.SetVarRaw(vname, newobj)
	return i.Return(newobj)
}

func getDuration(i *Interp, code *TclObj) (int64, TclStatus) {
	start := time.Nanoseconds()
	rc := i.EvalObj(code)
	end := time.Nanoseconds()
	return (end - start), rc
}

func formatTime(ns int64) string {
	us := float(ns) / 1000
	if us < 1000 {
		return fmt.Sprintf("%v µs", us)
	}
	return fmt.Sprintf("%v ms", us/1000)
}

func tclTime(i *Interp, args []*TclObj) TclStatus {
	if len(args) == 1 {
		dur, rc := getDuration(i, args[0])
		if rc != kTclOK {
			return rc
		}
		return i.Return(fromStr(formatTime(dur)))
	} else if len(args) == 2 {
		count, err := args[1].AsInt()
		if err != nil {
			return i.Fail(err)
		}
		total := int64(0)
		for x := 0; x < count; x++ {
			dur, _ := getDuration(i, args[0])
			total += dur
		}
		avg := total / int64(count)
		return i.Return(fromStr(formatTime(avg) + " per iteration"))
	}
	return i.FailStr("wrong # args")
}

func tclFlush(i *Interp, args []*TclObj) TclStatus {
	if len(args) != 1 {
		return i.FailStr("wrong # args")
	}
	outfile, ok := i.chans[args[0].AsString()]
	if !ok {
		return i.FailStr("no such channel")
	}
	if fl, ok := outfile.(interface {
		Flush() os.Error
	}); ok {
		fl.Flush()
	}
	return i.Return(kNil)
}

func tclPuts(i *Interp, args []*TclObj) TclStatus {
	newline := true
	var s string
	file := i.chans["stdout"].(io.Writer)
	if len(args) == 1 {
		s = args[0].AsString()
	} else if len(args) == 2 || len(args) == 3 {
		if args[0].AsString() == "-nonewline" {
			newline = false
			args = args[1:]
		}
		if len(args) > 1 {
			outfile, ok := i.chans[args[0].AsString()]
			if !ok {
				return i.FailStr("wrong args")
			}
			file, ok = outfile.(io.Writer)
			if !ok {
				return i.FailStr("channel wasn't opened for writing")
			}
			args = args[1:]
		}
		s = args[0].AsString()
	}
	if newline {
		fmt.Fprintln(file, s)
	} else {
		fmt.Fprint(file, s)
	}
	return i.Return(kNil)
}

func tclGets(i *Interp, args []*TclObj) TclStatus {
	if len(args) != 1 && len(args) != 2 {
		return i.FailStr("gets: wrong # args")
	}
	ini, ok := i.chans[args[0].AsString()]
	if !ok {
		return i.FailStr("invalid channel: " + args[0].AsString())
	}
	in, ok_read := ini.(*bufio.Reader)
	if !ok_read {
		return i.FailStr("channel wasn't opened for reading")
	}
	str, e := in.ReadString('\n')
	eof := false
	if e != nil {
		if e != os.EOF {
			return i.Fail(e)
		}
		eof = true
	}
	if len(str) > 0 {
		str = str[0 : len(str)-1]
	}
	if len(args) == 2 {
		resname := args[1].AsString()
		i.SetVarRaw(resname, fromStr(str))
		retval := len(str)
		if eof {
			retval = -1
		}
		return i.Return(FromInt(retval))
	}
	return i.Return(fromStr(str))
}

func tclInfo(i *Interp, args []*TclObj) TclStatus {
	if len(args) == 0 {
		return i.FailStr("wrong # args")
	}
	option := args[0].AsString()
	switch option {
	case "exists":
		if len(args) != 2 {
			return i.FailStr("wrong # args")
		}
		vname := args[1].AsString()
		_, err := i.GetVarRaw(vname)
		if err != nil {
			i.ClearError()
		}
		return i.Return(FromBool(err == nil))
	case "globals":
		if len(args) != 1 {
			return i.FailStr("wrong # args")
		}
		m := i.GetVarMap(true)
		results := make([]*TclObj, len(m))
		ind := 0
		for vn, _ := range m {
			results[ind] = fromStr(vn)
			ind++
		}
		return i.Return(fromList(results))
	case "commands":
		if len(args) != 1 {
			return i.FailStr("wrong # args")
		}
		cmds := make([]*TclObj, len(i.cmds))
		ind := 0
		for n, _ := range i.cmds {
			cmds[ind] = fromStr(n)
			ind++
		}
		return i.Return(fromList(cmds))

	}
	return i.FailStr("bad option \"" + option + "\"")
}

func tclString(i *Interp, args []*TclObj) TclStatus {
	if len(args) < 2 {
		return i.FailStr("wrong # args")
	}
	cmd := args[0].AsString()
	str := args[1].AsString()
	switch cmd {
	case "bytelength":
		return i.Return(FromInt(len(str)))
	case "length":
		return i.Return(FromInt(utf8.RuneCountInString(str)))
	case "index":
		ind, e := args[2].AsInt()
		if e != nil {
			return i.Fail(e)
		}
		if ind >= len(str) {
			return i.Return(kNil)
		}
		return i.Return(fromStr(str[ind : ind+1]))
	case "trim":
		return i.Return(fromStr(strings.TrimSpace(str)))
	}
	return i.FailStr("bad option to \"string\": " + cmd)
}

func tclSource(i *Interp, args []*TclObj) TclStatus {
	if len(args) != 1 {
		return i.FailStr("wrong # args")
	}
	filename := args[0].AsString()
	file, e := os.Open(filename, os.O_RDONLY, 0)
	if e != nil {
		return i.Fail(e)
	}
	defer file.Close()
	cmds, pe := ParseCommands(file)
	if pe != nil {
		return i.Fail(pe)
	}
	return i.eval(cmds)
}

func splitWhen(s string, pred func(int) bool) []string {
	n := 0
	inside := false
	for _, rune := range s {
		was_inside := inside
		inside := !pred(rune)
		if inside && !was_inside {
			n++
		}
	}
	a := make([]string, n)
	na := 0
	start := -1
	for i, rune := range s {
		if pred(rune) {
			if start >= 0 {
				a[na] = s[start:i]
				na++
				start = -1
			}
		} else if start == -1 {
			start = i
		}
	}
	if start != -1 {
		a[na] = s[start:]
		na++
	}
	return a[0:na]
}

func oneof(s string, c int) bool {
	for _, ch := range s {
		if ch == c {
			return true
		}
	}
	return false
}

func tclSplit(i *Interp, args []*TclObj) TclStatus {
	if len(args) != 1 && len(args) != 2 {
		return i.FailStr("wrong # args")
	}
	sin := args[0].AsString()
	var strs []string
	if len(args) == 1 {
		strs = strings.Fields(sin)
	} else if len(args) == 2 {
		chars := args[1].AsString()
		if len(chars) == 0 {
			strs = strings.Split(sin, "", -1)
		} else {
			strs = splitWhen(sin,
				func(c int) bool { return oneof(chars, c) })
		}
	}
	results := make([]*TclObj, len(strs))
	for i, s := range strs {
		results[i] = fromStr(s)
	}
	return i.Return(fromList(results))
}

func tclLsearch(i *Interp, args []*TclObj) TclStatus {
	if len(args) != 2 {
		return i.FailStr("wrong # args")
	}
	lst, err := args[0].AsList()
	if err != nil {
		return i.Fail(err)
	}
	pat := args[1].AsString()
	for ind, v := range lst {
		if v.AsString() == pat {
			return i.Return(FromInt(ind))
		}
	}
	return i.Return(FromInt(-1))
}

func tclRename(i *Interp, args []*TclObj) TclStatus {
	if len(args) != 2 {
		return i.FailStr("wrong # args")
	}
	oldn, newn := args[0].AsString(), args[1].AsString()
	oldc, ok := i.cmds[oldn]
	if newn == "" {
		if !ok {
			return i.FailStr("can't delete command, doesn't exist")
		}
		i.SetCmd(oldn, nil)
	} else {
		if !ok {
			return i.FailStr("can't rename command, doesn't exist")
		}
		i.SetCmd(oldn, nil)
		i.SetCmd(newn, oldc)
	}
	return i.Return(kNil)
}

func tclApply(i *Interp, args []*TclObj) TclStatus {
	if len(args) < 1 {
		return i.FailStr("wrong # args")
	}
	lambda, e := args[0].AsList()
	if e != nil {
		return i.Fail(e)
	}
	if len(lambda) != 2 {
		return i.FailStr("invalid lambda")
	}
	sig, se := lambda[0].AsList()
	if se != nil {
		return i.Fail(se)
	}
	return makeProc(sig, lambda[1])(i, args[1:])
}

var tclBasicCmds = make(map[string]TclCmd)
