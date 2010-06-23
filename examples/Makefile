include $(GOROOT)/src/Make.$(GOARCH)

ALL=simple repl

all: $(ALL)

clean:
	rm -rf *.[68] $(ALL)

%: %.go
	$(GC) $*.go
	$(LD) -o $@ $*.$O
