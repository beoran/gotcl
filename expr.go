package gotcl

import (
	"os"
	"io"
	"unicode"
)

type eterm interface {
	String() string
	Eval(*Interp) TclStatus
}

type binop struct {
	op   string
	a, b eterm
}

type unop struct {
	op string
	v  eterm
}

func (u *unop) String() string {
	return "(" + u.op + " " + u.v.String() + ")"
}

func (u *unop) Eval(i *Interp) TclStatus {
	rc := u.v.Eval(i)
	if rc != kTclOK {
		return rc
	}
	return i.Return(FromBool(!i.retval.AsBool()))
}


type parens struct {
	term eterm
}


func (p *parens) Eval(i *Interp) TclStatus {
	return p.term.Eval(i)
}

func (p *parens) String() string {
	return p.term.String()
}


func callCmd(i *Interp, name string, args ...*TclObj) TclStatus {
	c := i.cmds[name]
	if c == nil {
		return i.FailStr("Not a command: " + name)
	}
	return c(i, args)
}

func (bb *binop) Eval(i *Interp) TclStatus {
	bb.a.Eval(i)
	a := i.retval
	bb.b.Eval(i)
	b := i.retval
	if i.err != nil {
		return i.Fail(i.err)
	}
	return callCmd(i, bb.op, a, b)
}

func (bb *binop) String() string {
	return "(" + bb.op + " " + bb.a.String() + " " + bb.b.String() + ")"
}

func gbalance(b eterm) eterm {
	bb, ok := b.(*binop)
	if ok {
		return balance(bb)
	}
	return b
}

var oplevel = map[string]int{
	"*": 3, "/": 3,
	"+": 2, "-": 2,
	"==": 1, "!=": 1,
	"&&": 0, "||": 0}

func opgt(a, b string) bool {
	al, aok := oplevel[a]
	bl, bok := oplevel[b]
	if !aok || !bok {
		return false
	}
	return al >= bl
}

func balance(b *binop) *binop {
	bb, ok := b.b.(*binop)
	if !ok {
		return b
	}
	if opgt(b.op, bb.op) {
		return &binop{bb.op, &binop{b.op, gbalance(b.a), gbalance(bb.a)}, gbalance(bb.b)}
	}
	return b
}

func ParseExpr(in io.Reader) (item eterm, err os.Error) {
	p := newParser(in)
	defer setError(&err)
	item = p.parseExpr()
	return
}

func (p *parser) parseExpr() eterm {
	res := p.parseExprTerm()
	p.eatWhile(isspace)
	if p.ch != -1 {
		if p.ch == ')' {
			return res
		}
		return p.parseBinOp(res)
	}
	return res
}

func istermchar(c int) bool {
	return unicode.IsDigit(c) || unicode.IsLetter(c) || c == '.' || c == '-'
}

func (p *parser) parseExprTerm() eterm {
	p.eatWhile(isspace)
	switch p.ch {
	case '(':
		p.advance()
		e := p.parseExpr()
		p.consumeRune(')')
		return &parens{e}
	case '$':
		return p.parseVarRef()
	case '!':
		return p.parseUnOp()
	case '[':
		return p.parseSubcommand()
	}
	txt := p.consumeWhile1(istermchar, "term")
	return &tliteral{strval: txt}
}

func (p *parser) parseOp() string {
	switch p.ch {
	case '*':
		p.advance()
		return "*"
	case '+':
		p.advance()
		return "+"
	case '-':
		p.advance()
		return "-"
	case '|':
		p.advance()
		p.consumeRune('|')
		return "||"
	case '&':
		p.advance()
		p.consumeRune('&')
		return "&&"
	case '!':
		p.advance()
		p.consumeRune('=')
		return "!="
	case '=':
		p.advance()
		p.consumeRune('=')
		return "=="
	case '>':
		p.advance()
		if p.ch == '=' {
			p.advance()
			return ">="
		}
		return ">"
	case '<':
		p.advance()
		if p.ch == '=' {
			p.advance()
			return "<="
		}
		return "<"
	case -1:
		p.fail("EOF")
	}
	p.fail("expected operand, got " + string(p.ch))
	return ""
}

func (p *parser) parseUnOp() eterm {
	p.eatWhile(isspace)
	p.consumeRune('!')
	return &unop{"!", p.parseExprTerm()}
}

func (p *parser) parseBinOp(a eterm) eterm {
	op := p.parseOp()
	p.eatWhile(isspace)
	return balance(&binop{op, a, p.parseExpr()})
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
