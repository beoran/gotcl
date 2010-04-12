set ::passcount 0
set ::current_test ""


proc assert { a op b args } {
    if [$op $a $b] {
        incr ::passcount
        puts -nonewline "."
    } else {
        set extra ""
        if { [string length $args] != 0 } {
            set extra " ($args)"
        }
        error "{$a} doesn't $op {$b}$extra"
    }
}

proc assert_noerr code {
    set ev [catch [list uplevel $code] msg]
    assert $ev == 0
}

proc test {name body} {
    set ::current_test $name
    if [catch $body msg] {
        puts "${::current_test}: $msg"
    }
    set ::current_test ""
}

proc bench {code} {
    set res [uplevel "time {$code} 8"]
    puts "[string trim $code]: $res"
}
