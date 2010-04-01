include $(GOROOT)/src/Make.$(GOARCH)
TARG=consalus/gotcl
GOFILES=\
        gotcl.go\
        commands.go\

include $(GOROOT)/src/Make.pkg
