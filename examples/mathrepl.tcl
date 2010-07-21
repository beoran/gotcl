proc eval_printer {ch} {
    forchan exp $ch {
        catch {expr $exp} res
        puts $res
    }
}

set evalchan [newchan]
go eval_printer $evalchan

while {[gets stdin line] >= 0} {
    # ignore comments
    if {![string match #* $line]} {
        if {[string length $line] > 0} {
            sendchan $evalchan $line
        }
    }
}
closechan $evalchan
