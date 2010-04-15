package gotcl

import (
	"testing"
)

func tcheck(a, b string, should_match bool, t *testing.T) {
	if GlobMatch(a, b) != should_match {
		s := "should've"
		if !should_match {
			s = "should not have"
		}
		t.Error(a + " " + s + " matched " + b)
	}
}

func TestBasic(t *testing.T) {
	check := func(a, b string) { tcheck(a, b, true, t) }
	check_not := func(a, b string) { tcheck(a, b, false, t) }
	check("c?t", "cat")
	check("ca*", "cat")
	check("c*", "cat")
	check("c*at", "cat")
	check("c*t", "cat")
	check("???", "cat")
	check("a*cd", "abdddddbdbdbdbdbdbdbcd")
	check_not("a*dc", "abdddddbdbdbdbdbdbdbcd")
	check_not(`a\*b`, "acb")
	check(`a\*b`, "a*b")
	check("λ*", "λxxxx")
	check("λ?λ", "λλλ")
}
