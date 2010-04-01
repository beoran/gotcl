package gotcl

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"
)

type parser struct {
	data    *bufio.Reader
	tmpbuf  *bytes.Buffer
	errchan chan os.Error
	ch      int
}

func newParser(input io.Reader) *parser {
	p := &parser{data: bufio.NewReader(input), errchan: make(chan os.Error)}
	p.tmpbuf = bytes.NewBuffer(make([]byte, 0, 1024))
	p.advance()
	return p
}

func isspace(c int) bool    { return c == ' ' || c == '\n' || c == '\t' || c == '\r' }
func issepspace(c int) bool { return c == '\t' || c == ' ' }
func isword(c int) bool {
	switch c {
	case '{', '}', '[', ']', ';', '$', '"', '(', ')':
		return false
	}
	return !isspace(c)
}

func (p *parser) fail(s string) {
	p.errchan <- os.NewError(s)
	runtime.Goexit()
}

func (p *parser) advance() (result int) {
	if p.ch == -1 {
		p.fail("unexpected EOF")
	}
	result = p.ch
	r, _, e := p.data.ReadRune()
	if e != nil {
		if e != os.EOF {
			p.fail(e.String())
		}
		p.ch = -1
	} else {
		p.ch = r
	}
	return
}

func (p *parser) consumeWhile1(fn func(int) bool, desc string) string {
	p.tmpbuf.Reset()
	for p.ch != -1 && fn(p.ch) {
		p.tmpbuf.WriteRune(p.advance())
	}
	res := p.tmpbuf.String()
	if len(res) == 0 {
		got := string(p.ch)
		if p.ch == -1 {
			got = "EOF"
		}
		p.fail("expected " + desc + ", got " + got)
	}
	return res
}

func (p *parser) consumeRune(rune int) {
	if p.ch != rune {
		p.fail("Didn't start with " + string(rune) + " (" + string(p.ch) + ")")
	}
	p.advance()
}

func (p *parser) eatWhile(fn func(int) bool) {
	for p.ch != -1 && fn(p.ch) {
		p.advance()
	}
}

type tliteral struct {
	strval string
	tval   *TclObj
}

func (l *tliteral) String() string { return l.strval }
func (l *tliteral) Eval(i *Interp) *TclObj {
	if l.tval == nil {
		l.tval = fromStr(l.strval)
	}
	return l.tval
}

func (p *parser) parseSimpleWord() *tliteral {
	p.tmpbuf.Reset()
	prev_esc := false
	for p.ch != -1 {
		if p.ch == '\\' && !prev_esc {
			prev_esc = true
			p.advance()
		} else if prev_esc || isword(p.ch) {
			c := p.advance()
			if prev_esc {
				p.tmpbuf.WriteString(escaped(c))
				prev_esc = false
			} else {
				p.tmpbuf.WriteRune(c)
			}
		} else {
			break
		}
	}
	res := p.tmpbuf.String()
	if len(res) == 0 {
		got := "EOF"
		if p.ch != -1 {
			got = string(p.ch)
		}
		p.fail("expected word, got " + got)
	}
	return &tliteral{strval: res}
}

type subcommand struct {
	cmd Command
}

func (s *subcommand) String() string { return "[" + s.cmd.String() + "]" }
func (s *subcommand) Eval(i *Interp) *TclObj {
	i.evalCmd(s.cmd)
	return i.retval
}

type block struct {
	strval string
	tval   *TclObj
}

func (b *block) String() string { return "{" + b.strval + "}" }

func (b *block) Eval(i *Interp) *TclObj {
	if b.tval == nil {
		b.tval = fromStr(b.strval)
	}
	return b.tval
}

func (p *parser) parseSubcommand() *subcommand {
	p.consumeRune('[')
	res := make([]TclTok, 0, 16)
	p.eatWhile(issepspace)
	for p.ch != ']' {
		appendttok(&res, p.parseToken())
		p.eatWhile(issepspace)
	}
	p.consumeRune(']')
	return &subcommand{Command{res}}
}

func (p *parser) parseBlockData() string {
	p.consumeRune('{')
	nest := 0
	p.tmpbuf.Reset()
	for {
		switch p.ch {
		case '\\':
			p.tmpbuf.WriteRune(p.advance())
		case '{':
			nest++
		case '}':
			if nest == 0 {
				p.advance()
				return p.tmpbuf.String()
			}
			nest--
		case -1:
			p.fail("unclosed block")
		}
		p.tmpbuf.WriteRune(p.advance())
	}
	return "" // never happens.
}

func (p *parser) parseBlock() *block { return &block{strval: p.parseBlockData()} }

type strlit struct {
	toks []littok
}

func (t strlit) String() string {
	res := bytes.NewBufferString(`"`)
	for _, tok := range t.toks {
		if tok.kind == kRaw {
			res.WriteString(tok.value)
		} else if tok.kind == kVar {
			res.WriteString(tok.varref.String())
		} else if tok.kind == kSubcmd {
			res.WriteString(tok.subcmd.String())
		}
	}
	res.WriteString(`"`)
	return res.String()
}

func (t strlit) Eval(i *Interp) *TclObj {
	res := bytes.NewBufferString("")
	for _, tok := range t.toks {
		res.WriteString(tok.Eval(i))
	}
	return fromStr(res.String())
}


func (p *parser) parseVarRef() varRef {
	p.consumeRune('$')
	if p.ch == '{' {
		return toVarRef(p.parseBlockData())
	}
	global := false
	if p.ch == ':' {
		p.advance()
		p.consumeRune(':')
		global = true
	}
	return varRef{is_global: global, name: p.consumeWhile1(isword, "variable name")}
}

type varRef struct {
	is_global bool
	name      string
}

func (v varRef) Eval(i *Interp) *TclObj {
	x, _ := i.GetVar(v)
	return x
}

func (v varRef) String() string {
	str := v.name
	if v.is_global {
		str = "::" + str
	}
	return "$" + str
}

func toVarRef(s string) varRef {
	global := false
	if strings.HasPrefix(s, "::") {
		s = s[2:]
		global = true
	}
	return varRef{name: s, is_global: global}
}

type Command struct {
	words []TclTok
}

func (c *Command) String() string {
	result := ""
	first := true
	for _, w := range c.words {
		if first {
			first = false
		} else {
			result += " "
		}
		result += w.String()
	}
	return result
}

type TclTok interface {
	String() string
	Eval(i *Interp) *TclObj
}

const (
	kRaw = iota
	kVar
	kSubcmd
)

type littok struct {
	kind   int
	value  string
	varref *varRef
	subcmd *subcommand
}

func (lt *littok) Eval(i *Interp) string {
	switch lt.kind {
	case kRaw:
		return lt.value
	case kVar:
		return lt.varref.Eval(i).AsString()
	case kSubcmd:
		return lt.subcmd.Eval(i).AsString()
	}
	panic("unrecognized kind")
}

func realloc_slice(sl interface{}, newcap int) interface{} {
	s := reflect.NewValue(sl).(*reflect.SliceValue)
	if newcap <= s.Cap() {
		return sl
	}
	t := reflect.Typeof(sl).(*reflect.SliceType)
	nv := reflect.MakeSlice(t, s.Len(), newcap)
	reflect.ArrayCopy(nv, s)
	s.Set(nv)
	return s.Interface()
}

func appendtok(tx *[]littok, t littok) {
	oldlen := len(*tx)
	if oldlen == cap(*tx) {
		newcap := (cap(*tx) + 1) * 2
		*tx = realloc_slice(*tx, newcap).([]littok)
	}
	*tx = (*tx)[0 : oldlen+1]
	(*tx)[oldlen] = t
}

var escMap = map[int]string{
	'n': "\n", 't': "\t", 'a': "\a", 'v': "\v", 'r': "\r"}

func escaped(r int) string {
	if v, ok := escMap[r]; ok {
		return v
	}
	return string(r)
}

func (p *parser) parseStringLit() strlit {
	p.consumeRune('"')
	accum := bytes.NewBuffer(make([]byte, 0, 256))
	toks := make([]littok, 0, 8)
	record_accum := func() {
		if accum.Len() != 0 {
			appendtok(&toks, littok{kind: kRaw, value: accum.String()})
			accum.Reset()
		}
	}
	for {
		switch p.ch {
		case '"':
			record_accum()
			p.advance()
			return strlit{toks}
		case '$':
			record_accum()
			vref := p.parseVarRef()
			appendtok(&toks, littok{kind: kVar, varref: &vref})
		case '[':
			record_accum()
			subcmd := p.parseSubcommand()
			appendtok(&toks, littok{kind: kSubcmd, subcmd: subcmd})
		case '\\':
			p.advance()
			accum.WriteString(escaped(p.advance()))
		case -1:
			p.fail("Unexpected EOF, wanted \"")
		default:
			accum.WriteRune(p.advance())
		}
	}
	panic("unreachable")
}

func isEol(ch int) bool {
	switch ch {
	case -1, ';', '\n':
		return true
	}
	return false
}

func (p *parser) eatExtra() {
	p.eatWhile(isspace)
	for p.ch == ';' {
		p.consumeRune(';')
		p.eatWhile(isspace)
	}
}

func (p *parser) parseComment() {
	p.consumeRune('#')
	p.eatWhile(func(c int) bool { return c != '\n' })
}

func appendcmd(tx *[]Command, t Command) {
	oldlen := len(*tx)
	if oldlen == cap(*tx) {
		newcap := (cap(*tx) + 1) * 2
		*tx = realloc_slice(*tx, newcap).([]Command)
	}
	*tx = (*tx)[0 : oldlen+1]
	(*tx)[oldlen] = t
}

func (p *parser) parseCommands() []Command {
	p.eatWhile(isspace)
	res := make([]Command, 0, 32)
	for p.ch != -1 {
		if p.ch == '#' {
			p.parseComment()
		} else {
			appendcmd(&res, p.parseCommand())
		}
		p.eatExtra()
	}
	return res
}

func appendttok(tx *[]TclTok, t TclTok) {
	oldlen := len(*tx)
	if oldlen == cap(*tx) {
		newcap := (cap(*tx) + 1) * 2
		*tx = realloc_slice(*tx, newcap).([]TclTok)
	}
	*tx = (*tx)[0 : oldlen+1]
	(*tx)[oldlen] = t
}

func (p *parser) parseList() []TclTok {
	res := make([]TclTok, 0, 32)
	for p.ch != -1 {
		p.eatWhile(isspace)
		if p.ch == -1 {
			break
		}
		appendttok(&res, p.parseListToken())
	}
	return res
}

func notspace(c int) bool { return !isspace(c) }

func (p *parser) parseListToken() TclTok {
	p.eatWhile(isspace)
	switch p.ch {
	case '{':
		return &tliteral{strval: p.parseBlockData()}
	case '"':
		return p.parseStringLit()
	}
	return &tliteral{strval: p.consumeWhile1(notspace, "word")}
}

func (p *parser) parseCommand() Command {
	res := make([]TclTok, 0, 16)
	appendttok(&res, p.parseToken())
	p.eatWhile(issepspace)
	for !isEol(p.ch) {
		appendttok(&res, p.parseToken())
		p.eatWhile(issepspace)
	}
	return Command{res}
}

func (p *parser) parseToken() TclTok {
	p.eatWhile(isspace)
	switch p.ch {
	case '[':
		return p.parseSubcommand()
	case '{':
		return p.parseBlock()
	case '"':
		return p.parseStringLit()
	case '$':
		return p.parseVarRef()
	}
	return p.parseSimpleWord()
}

func ParseList(in io.Reader) ([]TclTok, os.Error) {
	p := newParser(in)
	var items []TclTok
	go func() {
		items = p.parseList()
		p.errchan <- nil
	}()
	e := <-p.errchan
	return items, e
}

func ParseCommands(in io.Reader) ([]Command, os.Error) {
	p := newParser(in)
	var cmds []Command
	go func() {
		cmds = p.parseCommands()
		p.errchan <- nil
	}()
	e := <-p.errchan
	return cmds, e
}

type TclStatus int

const (
	kTclOK       TclStatus = iota
	kTclErr      TclStatus = iota
	kTclReturn   TclStatus = iota
	kTclBreak    TclStatus = iota
	kTclContinue TclStatus = iota
)

type framelink struct {
	frame *stackframe
	name  string
}

type varEntry struct {
	obj  *TclObj
	link *framelink
}

type VarMap map[string]varEntry

type stackframe struct {
	vars VarMap
	next *stackframe
}

func newstackframe(tail *stackframe) *stackframe {
	return &stackframe{make(VarMap), tail}
}
func (s *stackframe) up() *stackframe { return s.next }

type Interp struct {
	cmds   map[string]TclCmd
	chans  map[string]interface{}
	frame  *stackframe
	retval *TclObj
	err    os.Error
}

func (i *Interp) Return(val *TclObj) TclStatus {
	i.retval = val
	return kTclOK
}

func (i *Interp) Fail(err os.Error) TclStatus {
	i.err = err
	return kTclErr
}

func (i *Interp) FailStr(msg string) TclStatus {
	return i.Fail(os.NewError(msg))
}

type TclObj struct {
	value   *string
	intval  *int
	listval []*TclObj
	cmdsval []Command
	vrefval *varRef
}


func (t *TclObj) AsString() string {
	if t.value == nil {
		if t.intval != nil {
			v := strconv.Itoa(*t.intval)
			t.value = &v
		} else if t.listval != nil {
			str := bytes.NewBufferString("")
			for ind, i := range t.listval {
				if ind != 0 {
					str.WriteString(" ")
				}
				sv := i.AsString()
				should_bracket := strings.IndexAny(sv, " \t\n\v") != -1 || len(sv) == 0
				if should_bracket {
					str.WriteString("{")
				}
				str.WriteString(sv)
				if should_bracket {
					str.WriteString("}")
				}
			}
			ss := str.String()
			t.value = &ss
		} else {
			panic("unspecified string")
		}
	}
	return *t.value
}

func (t *TclObj) AsInt() (int, os.Error) {
	if t.intval == nil {
		v, e := strconv.Atoi(*t.value)
		if e != nil {
			return 0, os.NewError("expected integer but got \"" + *t.value + "\"")
		}
		t.intval = &v
	}
	return *t.intval, nil
}

func (t *TclObj) AsCmds() ([]Command, os.Error) {
	if t.cmdsval == nil {
		c, e := ParseCommands(strings.NewReader(t.AsString()))
		if e != nil {
			return nil, e
		}
		t.cmdsval = c
	}
	return t.cmdsval, nil
}

func (t *TclObj) AsBool() bool {
	iv, err := t.AsInt()
	if err != nil {
		return true
	}
	return iv != 0
}

func (t *TclObj) asVarRef() varRef {
	if t.vrefval == nil {
		vr := toVarRef(t.AsString())
		t.vrefval = &vr
	}
	return *t.vrefval
}

func fromStr(s string) *TclObj { return &TclObj{value: &s} }
func FromInt(i int) *TclObj    { return &TclObj{intval: &i} }

func FromStr(s string) *TclObj { return fromStr(s) }

func FromList(l []string) *TclObj {
	vl := make([]*TclObj, len(l))
	for i, s := range l {
		vl[i] = fromStr(s)
	}
	return fromList(vl)
}

var kTrue = FromInt(1)
var kFalse = FromInt(0)

func FromBool(b bool) *TclObj {
	if b {
		return kTrue
	}
	return kFalse
}

var kNil = fromStr("")

func fromList(items []*TclObj) *TclObj { return &TclObj{listval: items} }

func (t *TclObj) AsList() ([]*TclObj, os.Error) {
	if t.listval == nil {
		var e os.Error
		t.listval, e = parseList(t.AsString())
		if e != nil {
			return nil, e
		}
	}
	return t.listval, nil
}

func parseList(txt string) ([]*TclObj, os.Error) {
	lst, err := ParseList(strings.NewReader(txt))
	if err != nil {
		return nil, err
	}
	result := make([]*TclObj, len(lst))
	for i, li := range lst {
		result[i] = fromStr(li.String())
	}
	return result, nil
}

func (i *Interp) EvalObj(obj *TclObj) TclStatus {
	cmds, e := obj.AsCmds()
	if e != nil {
		return i.Fail(e)
	}
	return i.eval(cmds)
}

type argsig struct {
	name string
	def  *TclObj
}

func (i *Interp) bindArgs(vnames []argsig, args []*TclObj) os.Error {
	lastind := len(vnames) - 1
	for ix, vn := range vnames {
		if ix == lastind && vn.name == "args" {
			i.SetVarRaw(vn.name, fromList(args[ix:]))
			return nil
		} else if ix >= len(args) {
			if vn.def == nil {
				return os.NewError("arg count mismatch")
			}
			i.SetVarRaw(vn.name, vn.def)
		} else {
			i.SetVarRaw(vn.name, args[ix])
		}
	}
	return nil
}


func makeProc(name string, sig []*TclObj, body *TclObj) TclCmd {
	cmds, ce := body.AsCmds()
	if ce != nil {
		return func(i *Interp, args []*TclObj) TclStatus { return i.Fail(ce) }
	}
	sigs := make([]argsig, len(sig))
	for i, a := range sig {
		sl, lerr := a.AsList()
		if lerr == nil && len(sl) == 2 {
			sigs[i] = argsig{sl[0].AsString(), sl[1]}
		} else {
			sigs[i] = argsig{name: a.AsString()}
		}
	}
	return func(i *Interp, args []*TclObj) TclStatus {
		i.frame = newstackframe(i.frame)
		if be := i.bindArgs(sigs, args); be != nil {
			i.frame = i.frame.up()
			return i.Fail(be)
		}
		rc := i.eval(cmds)
		if rc == kTclReturn {
			rc = kTclOK
		}
		i.frame = i.frame.up()
		return rc
	}
}

func tclProc(i *Interp, args []*TclObj) TclStatus {
	if len(args) != 3 {
		return i.FailStr("wrong # args")
	}
	name := args[0].AsString()
	sig, err := args[1].AsList()
	if err != nil {
		return i.Fail(err)
	}
	body := args[2]
	i.SetCmd(name, makeProc(name, sig, body))
	return i.Return(kNil)
}

var tclStdin = bufio.NewReader(os.Stdin)
var tclStdout = bufio.NewWriter(os.Stdout)

func NewInterp() *Interp {
	i := new(Interp)
	i.cmds = make(map[string]TclCmd)
	i.frame = newstackframe(nil)
	i.chans = make(map[string]interface{})
	i.chans["stdin"] = tclStdin
	i.chans["stdout"] = tclStdout
	i.chans["stderr"] = bufio.NewWriter(os.Stderr)

	for n, f := range tclBasicCmds {
		i.SetCmd(n, f)
	}

	i.SetCmd("proc", tclProc)
	i.SetCmd("error", func(ni *Interp, args []*TclObj) TclStatus { return i.FailStr(args[0].AsString()) })
	return i
}

type TclCmd func(*Interp, []*TclObj) TclStatus

func (i *Interp) SetCmd(name string, cmd TclCmd) {
	if cmd == nil {
		i.cmds[name] = nil, false
	} else {
		i.cmds[name] = cmd
	}
}

func (i *Interp) eval(cmds []Command) TclStatus {
	for ind, c := range cmds {
		res := i.evalCmd(c)
		if res != kTclOK {
			return res
		}
		if ind == len(cmds)-1 {
			return kTclOK
		}
	}
	return kTclOK
}

func (i *Interp) GetVarMap(global bool) VarMap {
	f := i.frame
	if global {
		for f.next != nil {
			f = f.next
		}
	}
	return f.vars
}

func (i *Interp) LinkVar(theirs, mine string) {
	theirf := i.frame.up()
	m := i.GetVarMap(false)
	m[mine] = varEntry{link: &framelink{theirf, theirs}}
}

func (i *Interp) SetVarRaw(name string, val *TclObj) {
	i.SetVar(toVarRef(name), val)
}

func (i *Interp) SetVar(vr varRef, val *TclObj) {
	m := i.GetVarMap(vr.is_global)
	if val == nil {
		m[vr.name] = varEntry{}, false
	} else {
		n := vr.name
		old, ok := m[n]
		for ok && old.link != nil {
			m = old.link.frame.vars
			n = old.link.name
			old, ok = m[n]
		}
		m[n] = varEntry{obj: val}
	}
}

func (i *Interp) GetVarRaw(name string) (*TclObj, bool) {
	return i.GetVar(toVarRef(name))
}

func (i *Interp) GetVar(vr varRef) (*TclObj, bool) {
	v, ok := i.GetVarMap(vr.is_global)[vr.name]
	if !ok {
		i.FailStr("variable not found: " + vr.String())
		return nil, false
	}
	for v.link != nil {
		v, ok = v.link.frame.vars[v.link.name]
		if !ok {
			i.FailStr("variable not found: " + vr.String())
			return nil, false
		}
	}
	return v.obj, ok
}

func evalArgs(i *Interp, toks []TclTok) []*TclObj {
	res := make([]*TclObj, len(toks))
	for ind, t := range toks {
		res[ind] = t.Eval(i)
		if i.err != nil {
			break
		}
	}
	return res
}

func (i *Interp) ClearError() { i.err = nil }

func (i *Interp) evalCmd(cmd Command) TclStatus {
	if len(cmd.words) == 0 {
		return i.Return(kNil)
	}
	args := evalArgs(i, cmd.words)
	if i.err != nil {
		return kTclErr
	}
	fname := args[0].AsString()
	if f, ok := i.cmds[fname]; ok {
		return f(i, args[1:])
	}
	return i.FailStr("command not found: " + fname)
}

func (i *Interp) EvalString(s string) (*TclObj, os.Error) {
    return i.Run(strings.NewReader(s))
}

func (i *Interp) Run(in io.Reader) (*TclObj, os.Error) {
	cmds, e := ParseCommands(in)
	if e != nil {
		return nil, e
	}
	r := i.eval(cmds)
	if r == kTclOK {
		if i.retval == nil {
			return kNil, nil
		}
		return i.retval, nil
	}
	if r != kTclOK && i.err == nil {
		i.err = os.NewError("uncaught error: " + strconv.Itoa(int(r)))
	}
	return nil, i.err
}
