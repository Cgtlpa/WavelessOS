CC ?= gcc
CFLAGS ?= -Os -static -Wall

all: init

init: init.o service.o
	$(CC) $(CFLAGS) -o $@ $^

init.o: init.c service.h
service.o: service.c service.h

clean:
	rm -f init *.o

install: init
	mkdir -p $(DESTDIR)/sbin
	cp init $(DESTDIR)/sbin/init

.PHONY: all clean install
