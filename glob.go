package gotcl

import (
	"utf8"
)

func uncons(s string) (int, string) {
	head, sz := utf8.DecodeRuneInString(s)
	if head == utf8.RuneError {
		return head, ""
	}
	return head, s[sz:]
}

func GlobMatch(pat, str string) bool {
	esc := false
	for pat != "" {
		ph, rest := uncons(pat)
		switch {
		case ph == '?' && !esc:
			if str == "" {
				return false
			}
			_, str = uncons(str)
		case ph == '\\' && !esc:
			esc = true
		case ph == '*' && !esc:
			if rest == "" {
				return true
			}
			ss := str
			for ss != "" {
				if GlobMatch(rest, ss) {
					return true
				}
				_, ss = uncons(ss)
			}
			return false
		default:
			esc = false
			if str == "" {
				return false
			}
			var sh int
			sh, str = uncons(str)
			if sh != ph {
				return false
			}
		}
		pat = rest
	}
	return str == ""
}
