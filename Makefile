include $(GOROOT)/src/Make.$(GOARCH)
TARG=gotcl
GOFILES=\
        gotcl.go\
        commands.go\

include $(GOROOT)/src/Make.pkg
