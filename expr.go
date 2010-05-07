package gotcl

import (
	"os"
	"unicode"
)

type eterm interface {
	String() string
	Eval(*Interp) TclStatus
}

type binOpNode struct {
	op   *binaryOp
	a, b eterm
}

type unOpNode struct {
	op int
	v  eterm
}

func (u *unOpNode) String() string {
	return "(" + string(u.op) + " " + u.v.String() + ")"
}

func (u *unOpNode) Eval(i *Interp) TclStatus {
	rc := u.v.Eval(i)
	if rc != kTclOK {
		return rc
	}
	if u.op == '!' {
		return i.Return(FromBool(!i.retval.AsBool()))
	} else if u.op == '~' {
		iv, e := i.retval.AsInt()
		if e != nil {
			return i.Fail(e)
		}
		return i.Return(FromInt(^iv))
	}
	return i.FailStr("invalid unary operator")
}

type parenNode struct {
	term eterm
}

func (p *parenNode) Eval(i *Interp) TclStatus {
	return p.term.Eval(i)
}

func (p *parenNode) String() string {
	return p.term.String()
}

func (bb *binOpNode) Eval(i *Interp) TclStatus {
	bb.a.Eval(i)
	a := i.retval
	bb.b.Eval(i)
	b := i.retval
	if i.err != nil {
		return i.Fail(i.err)
	}
	r, e := bb.op.action(a, b)
	if e != nil {
		return i.Fail(e)
	}
	return i.Return(r)
}

func (bb *binOpNode) String() string {
	return "(" + bb.op.name + " " + bb.a.String() + " " + bb.b.String() + ")"
}

type binaryOp struct {
	name       string
	precedence int
	action     func(*TclObj, *TclObj) (*TclObj, os.Error)
}

var BinOps = []*binaryOp{
	plusOp, minusOp, timesOp, xorOp, divideOp, lshiftOp, rshiftOp,
	equalsOp, notEqualsOp, andOp, orOp, gtOp, gteOp, ltOp, lteOp,
}

var plusOp = &binaryOp{"+", 2,
	func(a, b *TclObj) (*TclObj, os.Error) {
		i1, i2, e := asInts(a, b)
		return FromInt(i1 + i2), e
	}}
var minusOp = &binaryOp{"-", 2,
	func(a, b *TclObj) (*TclObj, os.Error) {
		i1, i2, e := asInts(a, b)
		return FromInt(i1 - i2), e
	}}
var timesOp = &binaryOp{"*", 3,
	func(a, b *TclObj) (*TclObj, os.Error) {
		i1, i2, e := asInts(a, b)
		return FromInt(i1 * i2), e
	}}
var xorOp = &binaryOp{"^", 3,
	func(a, b *TclObj) (*TclObj, os.Error) {
		i1, i2, e := asInts(a, b)
		return FromInt(i1 ^ i2), e
	}}
var divideOp = &binaryOp{"*", 3,
	func(a, b *TclObj) (*TclObj, os.Error) {
		i1, i2, e := asInts(a, b)
		return FromInt(i1 / i2), e
	}}
var lshiftOp = &binaryOp{"<<", 4,
	func(a, b *TclObj) (*TclObj, os.Error) {
		i1, i2, e := asInts(a, b)
		return FromInt(i1 << uint(i2)), e
	}}
var rshiftOp = &binaryOp{">>", 4,
	func(a, b *TclObj) (*TclObj, os.Error) {
		i1, i2, e := asInts(a, b)
		return FromInt(i1 >> uint(i2)), e
	}}
var equalsOp = &binaryOp{"==", 1,
	func(a, b *TclObj) (*TclObj, os.Error) {
		return FromBool(a.AsString() == b.AsString()), nil
	}}
var notEqualsOp = &binaryOp{"!=", 1,
	func(a, b *TclObj) (*TclObj, os.Error) {
		return FromBool(a.AsString() != b.AsString()), nil
	}}
var andOp = &binaryOp{"&&", 0,
	func(a, b *TclObj) (*TclObj, os.Error) {
		return FromBool(a.AsBool() && b.AsBool()), nil
	}}
var orOp = &binaryOp{"||", 0,
	func(a, b *TclObj) (*TclObj, os.Error) {
		return FromBool(a.AsBool() || b.AsBool()), nil
	}}
var gtOp = &binaryOp{">", -1,
	func(a, b *TclObj) (*TclObj, os.Error) {
		i1, i2, e := asInts(a, b)
		return FromBool(i1 > i2), e
	}}
var gteOp = &binaryOp{">=", -1,
	func(a, b *TclObj) (*TclObj, os.Error) {
		i1, i2, e := asInts(a, b)
		return FromBool(i1 >= i2), e
	}}
var ltOp = &binaryOp{"<", -1,
	func(a, b *TclObj) (*TclObj, os.Error) {
		i1, i2, e := asInts(a, b)
		return FromBool(i1 < i2), e
	}}
var lteOp = &binaryOp{"<=", -1,
	func(a, b *TclObj) (*TclObj, os.Error) {
		i1, i2, e := asInts(a, b)
		return FromBool(i1 <= i2), e
	}}

func gbalance(b eterm) eterm {
	bb, ok := b.(*binOpNode)
	if ok {
		return balance(bb)
	}
	return b
}

func balance(b *binOpNode) *binOpNode {
	bb, ok := b.b.(*binOpNode)
	if ok && b.op.precedence >= bb.op.precedence {
		return &binOpNode{bb.op,
			&binOpNode{b.op, gbalance(b.a), gbalance(bb.a)},
			gbalance(bb.b)}
	}
	return b
}

func ParseExpr(in RuneSource) (item eterm, err os.Error) {
	p := newParser(in)
	defer setError(&err)
	item = p.parseExpr()
	return
}

func (p *parser) parseExpr() eterm {
	res := p.parseExprTerm()
	p.eatWhile(unicode.IsSpace)
	if p.ch != -1 {
		if p.ch == ')' {
			return res
		}
		if p.ch == '?' {
			return p.parseTernaryIf(res)
		}
		return p.parseBinOpNode(res)
	}
	return res
}

type ternaryIfNode struct {
	cond, yes, no eterm
}

func (ti *ternaryIfNode) Eval(i *Interp) TclStatus {
	rc := ti.cond.Eval(i)
	if rc != kTclOK {
		return rc
	}
	v := i.retval
	if v.AsBool() {
		return ti.yes.Eval(i)
	}
	return ti.no.Eval(i)
}

func (ti *ternaryIfNode) String() string {
	return ti.cond.String() + " ? " + ti.yes.String() + " : " + ti.no.String()
}

func (p *parser) parseTernaryIf(cond eterm) *ternaryIfNode {
	p.consumeRune('?')
	p.eatWhile(unicode.IsSpace)
	yes := p.parseExprTerm()
	p.eatWhile(unicode.IsSpace)
	p.consumeRune(':')
	p.eatWhile(unicode.IsSpace)
	no := p.parseExprTerm()
	return &ternaryIfNode{cond, yes, no}
}

func istermchar(c int) bool {
	return unicode.IsDigit(c) || unicode.IsLetter(c) || c == '.' || c == '-'
}

func (p *parser) parseExprTerm() eterm {
	p.eatWhile(unicode.IsSpace)
	switch p.ch {
	case '(':
		p.advance()
		e := p.parseExpr()
		p.consumeRune(')')
		return &parenNode{e}
	case '$':
		return p.parseVarRef()
	case '!', '~':
		return p.parseUnOpNode()
	case '"':
		return p.parseStringLit()
	case '[':
		return p.parseSubcommand()
	}
	txt := p.consumeWhile1(istermchar, "term")
	return &tliteral{strval: txt}
}

func (p *parser) parseBinOp() *binaryOp {
	c := p.advance()
	switch c {
	case '*':
		return timesOp
	case '+':
		return plusOp
	case '-':
		return minusOp
	case '|':
		p.consumeRune('|')
		return orOp
	case 'e':
		p.consumeRune('q')
		return equalsOp
	case 'n':
		p.consumeRune('e')
		return notEqualsOp
	case '&':
		p.consumeRune('&')
		return andOp
	case '^':
		return xorOp
	case '!':
		p.consumeRune('=')
		return notEqualsOp
	case '=':
		p.consumeRune('=')
		return equalsOp
	case '>':
		if p.ch == '=' {
			p.advance()
			return gteOp
		} else if p.ch == '>' {
			p.advance()
			return rshiftOp
		}
		return gtOp
	case '<':
		if p.ch == '=' {
			p.advance()
			return lteOp
		} else if p.ch == '<' {
			p.advance()
			return lshiftOp
		}
		return ltOp
	case -1:
		p.fail("EOF")
	}
	p.fail("expected binary operator, got " + string(p.ch))
	return nil
}

func (p *parser) parseUnOpNode() *unOpNode {
	if p.ch != '!' && p.ch != '~' {
		p.fail("expected unary operator")
	}
	op := p.advance()
	return &unOpNode{op, p.parseExprTerm()}
}

func (p *parser) parseBinOpNode(a eterm) *binOpNode {
	op := p.parseBinOp()
	return balance(&binOpNode{op, a, p.parseExpr()})
}

func tclExpr(i *Interp, args []*TclObj) TclStatus {
	if len(args) == 0 {
		return i.FailStr("wrong # args")
	}
	var expr eterm
	var err os.Error
	if len(args) == 1 {
		expr, err = args[0].asExpr()
	} else {
		expr, err = concat(args).asExpr()
	}
	if err != nil {
		return i.Fail(err)
	}
	return expr.Eval(i)
}
