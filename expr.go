package gotcl

import (
	"os"
	"io"
	"unicode"
)

type eterm interface {
	String() string
	Eval(*Interp) *TclObj
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

func (u *unop) Eval(i *Interp) *TclObj {
	v := u.v.Eval(i)
	if i.err != nil {
		return nil
	}
	return FromBool(!v.AsBool())
}


type parens struct {
	term eterm
}


func (p *parens) Eval(i *Interp) *TclObj {
	return p.term.Eval(i)
}

func (p *parens) String() string {
	return p.term.String()
}


func Call(i *Interp, name string, args ...*TclObj) *TclObj {
	c := i.cmds[name]
	if c == nil {
		i.err = os.NewError("Not a command: " + name)
		return kNil
	}
	c(i, args)
	return i.retval
}

func (bb *binop) Eval(i *Interp) *TclObj {
	a := bb.a.Eval(i)
	b := bb.b.Eval(i)
	if i.err != nil {
		return nil
	}
	return Call(i, bb.op, a, b)
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

var oplevel = map[string]int{"*": 3, "/": 3, "+": 2, "-": 2, "==": 1, "&&": 0, "||": 0}

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
	p.fail("expected operand")
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
