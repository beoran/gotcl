package gotcl

import (
	"bufio"
	"bytes"
	"container/vector"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode"
	"utf8"
)

func tclSet(i *Interp, args []*TclObj) TclStatus {
	if len(args) == 0 || len(args) > 2 {
		return i.FailStr("wrong # args")
	}
	if len(args) == 2 {
		val := args[1]
		i.SetVar(args[0].AsVarRef(), val)
		return i.Return(val)
	}
	v, e := i.GetVar(args[0].AsVarRef())
	if e != nil {
		return i.Fail(e)
	}
	return i.Return(v)
}

func tclUnset(i *Interp, args []*TclObj) TclStatus {
	if len(args) == 0 {
		return i.FailStr("wrong # args")
	}
	i.SetVar(args[0].AsVarRef(), nil)
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

var uniqueNum int = 0

func tclOpen(i *Interp, args []*TclObj) TclStatus {
	if len(args) != 1 {
		return i.FailStr("wrong # args")
	}
	fname := args[0].AsString()
	ff, err := os.Open(fname, os.O_RDONLY, 0)
	if err != nil {
		return i.Fail(err)
	}
	channame := fmt.Sprintf("file%d", uniqueNum)
	uniqueNum++
	i.chans[channame] = bufio.NewReader(ff)
	return i.Return(FromStr(channame))
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
	vn := args[0].AsVarRef()
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
	if len(args) == 2 {
		val := kNil
		if r == kTclErr {
			val = FromStr(i.err.String())
		} else if r == kTclOK {
			val = i.retval
		}
		i.SetVar(args[1].AsVarRef(), val)
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
	rc := cond.Eval(i)
	if rc != kTclOK {
		return rc
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
		rc = i.EvalObj(next)
		if rc != kTclOK {
			return rc
		}
		if rc = testexpr.Eval(i); rc != kTclOK {
			return rc
		}
		cond = i.retval.AsBool()
	}
	return i.Return(kNil)
}

func tclForeach(i *Interp, args []*TclObj) TclStatus {
	if len(args) != 3 {
		return i.FailStr("wrong # args: should be \"foreach varName list body\"")
	}
	list, err := args[1].AsList()
	if err != nil {
		return i.Fail(err)
	}
	body := args[2]
	vlist, err := args[0].AsList()
	if err != nil {
		return i.Fail(err)
	}
	chunksz := len(vlist)
	if chunksz == 0 {
		return i.FailStr("foreach varlist is empty")
	}
	for len(list) > 0 {
		for ind, vn := range vlist {
			i.SetVar(vn.AsVarRef(), list[ind])
		}
		list = list[chunksz:]
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

func MakeCmd(fni interface{}) TclCmd {
	return to_cmd(fni)
}

func to_cmd(fni interface{}) TclCmd {
	switch fn := fni.(type) {
	case func(*Interp, []*TclObj) TclStatus:
		return fn
	case func(*TclObj, *TclObj) bool:
		return func(i *Interp, args []*TclObj) TclStatus {
			if len(args) != 2 {
				return i.FailStr("wrong # args")
			}
			return i.Return(FromBool(fn(args[0], args[1])))
		}
	case func(*Interp) *TclObj:
		return func(i *Interp, args []*TclObj) TclStatus {
			if len(args) != 0 {
				return i.FailStr("wrong # args")
			}
			v := fn(i)
			return i.Return(v)
		}

	case func(*TclObj, *TclObj) (*TclObj, os.Error):
		return func(i *Interp, args []*TclObj) TclStatus {
			if len(args) != 2 {
				return i.FailStr("wrong # args")
			}
			rv, e := fn(args[0], args[1])
			if e != nil {
				return i.Fail(e)
			}
			return i.Return(rv)
		}
	case func(string):
		return func(it *Interp, args []*TclObj) TclStatus {
			if len(args) != 1 {
				return it.FailStr("wrong # args")
			}
			fn(args[0].AsString())
			return it.Return(kNil)
		}
	case func(string) int:
		return func(it *Interp, args []*TclObj) TclStatus {
			if len(args) != 1 {
				return it.FailStr("wrong # args")
			}
			return it.Return(FromInt(fn(args[0].AsString())))
		}
	case func(string) string:
		return func(it *Interp, args []*TclObj) TclStatus {
			if len(args) != 1 {
				return it.FailStr("wrong # args")
			}
			return it.Return(FromStr(fn(args[0].AsString())))
		}
	case func(int):
		return func(it *Interp, args []*TclObj) TclStatus {
			if len(args) != 1 {
				return it.FailStr("wrong # args")
			}
			nv, _ := args[0].AsInt()
			fn(nv)
			return it.Return(kNil)
		}
	case func(string, string) bool:
		return func(it *Interp, args []*TclObj) TclStatus {
			if len(args) != 2 {
				return it.FailStr("wrong # args")
			}
			return it.Return(FromBool(fn(args[0].AsString(), args[1].AsString())))
		}
	case func(string) os.Error:
		return func(it *Interp, args []*TclObj) TclStatus {
			if len(args) != 1 {
				return it.FailStr("wrong # args")
			}
			e := fn(args[0].AsString())
			if e != nil {
				return it.Fail(e)
			}
			return it.Return(kNil)
		}
	case func(int, int) int:
		return func(i *Interp, args []*TclObj) TclStatus {
			if len(args) != 2 {
				return i.FailStr("wrong # args")
			}
			a, b, e := asInts(args[0], args[1])
			if e != nil {
				return i.Fail(e)
			}
			return i.Return(FromInt(fn(a, b)))
		}
	case func(int, int) bool:
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
	panic("Can't convert!")
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
	var result bytes.Buffer
	for ind, x := range args {
		if ind != 0 {
			result.WriteString(" ")
		}
		result.WriteString(strings.TrimSpace(x.AsString()))
	}
	return FromStr(result.String())
}

func tclEval(i *Interp, args []*TclObj) TclStatus {
	if len(args) == 0 {
		return i.FailStr("wrong # args")
	}
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
	vname := args[0].AsVarRef()
	v, ve := i.GetVar(vname)
	if ve != nil {
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
	i.SetVar(vname, newobj)
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
		return i.Return(FromStr(formatTime(dur)))
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
		return i.Return(FromStr(formatTime(avg) + " per iteration"))
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
		i.SetVar(args[1].AsVarRef(), FromStr(str))
		retval := len(str)
		if eof {
			retval = -1
		}
		return i.Return(FromInt(retval))
	}
	return i.Return(FromStr(str))
}

func getVarNameList(m VarMap) *TclObj {
	results := make([]*TclObj, len(m))
	ind := 0
	for vn, _ := range m {
		results[ind] = FromStr(vn)
		ind++
	}
	return fromList(results)
}

var infoEn = EnsembleSpec{
	"exists": varExists,
	"vars": func(i *Interp) *TclObj {
		return getVarNameList(i.GetVarMap(false))
	},
	"globals": func(i *Interp) *TclObj {
		return getVarNameList(i.GetVarMap(true))
	},
	"commands": getCmdNames,
}

func varExists(i *Interp, args []*TclObj) TclStatus {
	if len(args) != 1 {
		return i.FailStr("wrong # args")
	}
	_, err := i.GetVar(args[0].AsVarRef())
	return i.Return(FromBool(err == nil))
}

func getCmdNames(i *Interp) *TclObj {
	cmds := make([]*TclObj, len(i.cmds))
	ind := 0
	for n, _ := range i.cmds {
		cmds[ind] = FromStr(n)
		ind++
	}
	return fromList(cmds)
}

var stringEn = EnsembleSpec{
	"length":     utf8.RuneCountInString,
	"bytelength": func(s string) int { return len(s) },
	"trim":       strings.TrimSpace,
	"match":      GlobMatch,
	"index":      strIndex,
}

func strIndex(i *Interp, args []*TclObj) TclStatus {
	if len(args) != 2 {
		return i.FailStr("wrong # args")
	}
	str := args[0].AsString()
	ind, e := args[1].AsInt()
	if e != nil {
		if args[1].AsString() == "end" {
			ind = len(str) - 1
		} else {
			return i.Fail(e)
		}
	}
	if ind >= len(str) {
		return i.Return(kNil)
	}
	return i.Return(FromStr(str[ind : ind+1]))
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
	cmds, pe := ParseCommands(bufio.NewReader(file))
	if pe != nil {
		return i.Fail(pe)
	}
	return i.eval(cmds)
}

func splitWith(s string, fn func(int) bool) []string {
	var res vector.StringVector
	for {
		i := strings.IndexFunc(s, fn)
		if i == -1 {
			res.Push(s)
			break
		}
		res.Push(s[0:i])
		s = s[i+1:]
	}
	return []string(res)
}

func tclSplit(i *Interp, args []*TclObj) TclStatus {
	if len(args) != 1 && len(args) != 2 {
		return i.FailStr("wrong # args")
	}
	sin := args[0].AsString()
	var strs []string
	if len(args) == 1 {
		strs = splitWith(sin, unicode.IsSpace)
	} else if len(args) == 2 {
		chars := args[1].AsString()
		if len(chars) == 0 {
			strs = strings.Split(sin, "", -1)
		} else {
			strs = splitWith(sin,
				func(c int) bool { return strings.IndexRune(chars, c) != -1 })
		}
	}
	return i.Return(FromList(strs))
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

func init() {
	for _, o := range BinOps {
		tclBasicCmds[o.name] = to_cmd(o.action)
	}
	initCmds := map[string]TclCmd{
		"apply":    tclApply,
		"break":    tclBreak,
		"catch":    tclCatch,
		"concat":   tclConcat,
		"continue": tclContinue,
		"eval":     tclEval,
		"exit":     tclExit,
		"expr":     tclExpr,
		"flush":    tclFlush,
		"for":      tclFor,
		"foreach":  tclForeach,
		"gets":     tclGets,
		"if":       tclIf,
		"incr":     tclIncr,
		"info":     infoEn.MakeCmd(),
		"lappend":  tclLappend,
		"lindex":   tclLindex,
		"list":     tclList,
		"llength":  tclLlength,
		"lsearch":  tclLsearch,
		"open":     tclOpen,
		"puts":     tclPuts,
		"rename":   tclRename,
		"return":   tclReturn,
		"set":      tclSet,
		"source":   tclSource,
		"split":    tclSplit,
		"string":   stringEn.MakeCmd(),
		"time":     tclTime,
		"unset":    tclUnset,
		"uplevel":  tclUplevel,
		"upvar":    tclUpvar,
		"while":    tclWhile,
	}
	for k, v := range initCmds {
		tclBasicCmds[k] = v
	}
}
