package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

var options struct {
	pidfile string
}

func bail(status int, template string, args ...interface{}) {
	if status == 0 {
		fmt.Fprintf(os.Stdout, template, args...)
	} else {
		fmt.Fprintf(os.Stderr, template, args...)
	}
	os.Exit(status)
}

func writePid() error {
	f, err := os.OpenFile(options.pidfile, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(f, os.Getpid()); err != nil {
		return err
	}
	f.Close()
	return nil
}

func writelines(f *fileOpener, c chan []byte) {
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)

	kill := make(chan os.Signal, 1)
	signal.Notify(kill, syscall.SIGKILL)

	defer f.Close()

	for {
		select {
		case line, ok := <-c:
			if !ok { // end of file
				return
			}
			if _, err := f.Write(line); err != nil {
				f.Close()
				bail(1, "failed to write line: %v", err)
			}
		case <-hup:
			if err := f.Reopen(); err != nil {
				f.Close()
				bail(1, "unable to reopen file in writelines loop: %v", err)
			}
		case <-kill:
			return
		}
	}
}

type fileOpener struct {
	*os.File
	fname string
	flag  int
	mode  os.FileMode
}

func (f *fileOpener) Open() error {
	handle, err := os.OpenFile(f.fname, f.flag, f.mode)
	if err != nil {
		return fmt.Errorf("fileOpener unable to open file: %v", err)
	}
	f.File = handle
	return nil
}

func (f *fileOpener) Reopen() error {
	if err := f.Close(); err != nil {
		return fmt.Errorf("fileOpener unable to reopen file: %v", err)
	}
	if err := f.Open(); err != nil {
		return fmt.Errorf("fileOpener unable to reopen file: %v", err)
	}
	return nil
}

func (f *fileOpener) Close() error {
	if f.File == nil {
		return nil
	}
	return f.File.Close()
}

func main() {
	flag.Parse()

	if flag.NArg() < 1 {
		bail(1, "filename required")
	}
	fname := flag.Arg(0)

	f := &fileOpener{
		fname: fname,
		flag:  os.O_APPEND | os.O_CREATE | os.O_WRONLY,
		mode:  0644,
	}
	if err := f.Open(); err != nil {
		bail(1, "unable to open output file: %v", err)
	}
	defer f.Close()

	if err := writePid(); err != nil {
		bail(1, "unable to write pidfile: %v", err)
	}

	c := make(chan []byte)
	go writelines(f, c)

	r := bufio.NewReader(os.Stdin)
	for {
		line, err := r.ReadBytes('\n')
		switch err {
		case nil:
			c <- line
		case io.EOF:
			return
		default:
			f.Close()
			bail(2, "something is fucked: %v", err)
		}
	}
}

func init() {
	flag.StringVar(&options.pidfile, "pidfile", "./huppend.pid", "pidfile")
}
