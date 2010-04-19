package gotcl

import (
	"fmt"
	"sync"
)


var tclChans = make(map[string]chan *TclObj)

var chanindex = 0

func nextchanname() string {
	chanindex++
	return fmt.Sprintf("chan%d", chanindex)
}

func init() {
	for k, v := range tclChanCmds {
		tclBasicCmds[k] = v
	}
}

var chanMutex sync.Mutex

func makechan() string {
	name := nextchanname()
	ch := make(chan *TclObj)
	chanMutex.Lock()
	tclChans[name] = ch
	chanMutex.Unlock()
	return name
}

func getchan(name string) (res chan *TclObj) {
	chanMutex.Lock()
	res = tclChans[name]
	chanMutex.Unlock()
	return
}

func tclNewChan(i *Interp, args []*TclObj) TclStatus {
	if len(args) != 0 {
		return i.FailStr("wrong # args")
	}
	return i.Return(FromStr(makechan()))
}

func tclCloseChan(i *Interp, args []*TclObj) TclStatus {
	if len(args) != 1 {
		return i.FailStr("wrong # args")
	}
	name := args[0].AsString()
	ch := getchan(name)
	if ch == nil {
		return i.FailStr("not a chan: " + name)
	}
	close(ch)
	return i.Return(kNil)
}

func tclRecvChan(i *Interp, args []*TclObj) TclStatus {
	if len(args) != 1 {
		return i.FailStr("wrong # args")
	}
	name := args[0].AsString()
	ch := getchan(name)
	if ch == nil {
		return i.FailStr("not a chan: " + name)
	}
	v := <-ch
	if v == nil {
		v = kNil
	}
	return i.Return(v)
}

func tclSendChan(i *Interp, args []*TclObj) TclStatus {
	if len(args) != 2 {
		return i.FailStr("wrong # args")
	}
	name := args[0].AsString()
	ch := getchan(name)
	if ch == nil {
		return i.FailStr("not a chan: " + name)
	}
	ch <- args[1]
	return i.Return(kNil)
}


func tclGo(i *Interp, args []*TclObj) TclStatus {
	ni := new(Interp)
	ni.cmds = i.cmds
	ni.chans = i.chans
	ni.frame = newstackframe(nil)
	go func() {
		tclEval(ni, args)
		if ni.err != nil {
			fmt.Println(ni.err.String())
		}
	}()
	return i.Return(kNil)
}

func tclForChan(i *Interp, args []*TclObj) TclStatus {
	if len(args) != 3 {
		return i.FailStr("wrong # args")
	}
	vname := args[0].AsString()
	name := args[1].AsString()
	ch := getchan(name)
	if ch == nil {
		return i.FailStr("not a chan: " + name)
	}
	for v := range ch {
		i.SetVarRaw(vname, v)
		rc := i.EvalObj(args[2])
		if rc == kTclBreak {
			break
		} else if rc != kTclOK && rc != kTclContinue {
			return rc
		}
	}
	return i.Return(kNil)
}

var tclChanCmds = map[string]TclCmd{
	"go":        tclGo,
	"sendchan":  tclSendChan,
	"<-":        tclRecvChan,
	"newchan":   tclNewChan,
	"closechan": tclCloseChan,
	"forchan":   tclForChan,
}
