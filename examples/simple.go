package main

import "gotcl"

var code = `
proc sumto max {
    set sum 0
    for { set i 0 } { $i < $max } { incr i } {
        incr sum $i
    }
    return $sum
}

puts [sumto 5000]
`

func main() {
	gotcl.NewInterp().EvalString(code)
}
