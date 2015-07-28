Interpreter for a Tcl-like language as a Go library.

The goal isn't really to be a full-fledged Tcl interpreter, since that mostly involves implementing a bunch of commands with a pile of corner cases.
Instead, the goal is to have a safe, efficient, small, and extensible interpreter for
embedding.

For a trivial example of use, see http://code.google.com/p/gotcl/source/browse/examples/simple.go

Here's an example that uses the Go channel/goroutine wrappers: http://code.google.com/p/gotcl/source/browse/examples/chans.tcl

Working:
  * Parsing
  * expr
  * Arrays
  * {`*`} syntax.
  * The basics
  * Basic goroutines and channels in tcl.

Not supported:
  * floats
  * namespaces
  * lots of commands