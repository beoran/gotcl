proc gento {max chan} {
    for {set i 0} { $i < $max } { incr i } {
        sendchan $chan $i
    }
    closechan $chan
}

proc gen max {
    set ch [newchan]
    go gento $max $ch
    return $ch
} 

proc zip {ch1 ch2} {
    set out [newchan]
    set code { { ch1 ch2 res } {
        forchan v $ch1 {
            set v2 [<- $ch2]
            sendchan $res [+ $v $v2]
        }
        closechan $res
    }}
    go [list apply $code $ch1 $ch2 $out]
    return $out
}

forchan v [zip [gen 10] [gen 10]] {
    puts $v
}

proc sumchan ch {
    set res 0
    forchan v $ch {
        incr res $v
    }
    return $res
}
puts "sum: [sumchan [gen 500]]"
