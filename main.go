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
	lineBuf int
}

// writes out a message and then exits with the status coded provided by
// status.  Since bail calls os.Exit, defered functions are not run.
func bail(status int, template string, args ...interface{}) {
	if status == 0 {
		fmt.Fprintf(os.Stdout, template, args...)
	} else {
		fmt.Fprintf(os.Stderr, template, args...)
	}
	os.Exit(status)
}

// writes out the current process's pid to a file
func writePid() error {
	f, err := os.OpenFile(options.pidfile, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := fmt.Fprintln(f, os.Getpid()); err != nil {
		return err
	}
	return nil
}

// writes lines to the file handled by the fileopener f as they appear on the
// channel c.  when huppend receives a hup process, the fileopener f is
// reopened.
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
				fmt.Fprintf(os.Stderr, "unable to reopen file in writelines loop: %v\n", err)
			}
		case <-kill:
			return
		}
	}
}

// a fileopener wraps a file and provides a helper for reopening the file by
// path.
type fileOpener struct {
	*os.File

	// name of the file to be opened
	fname string

	// file create flags to be passed to os.Open when opening the file..  If
	// you don't include os.O_APPEND you're most likely doing it wrong.
	flag int

	// file permissions on file to be created, passed to os.Open.  If you don't
	// include os.O_CREATE in the flags then this is inconsequential since
	// we're not going to create new files anyway.
	mode os.FileMode
}

func (f *fileOpener) Open() error {
	handle, err := os.OpenFile(f.fname, f.flag, f.mode)
	if err != nil {
		return fmt.Errorf("fileOpener unable to open file: %v", err)
	}
	f.File = handle
	return nil
}

// closes and reopens the file found at the fileopener's specified path.  If
// the original file has moved, reopen will cause the fileopener to change which
// inode it points to.  If the original file has been moved or deleted, and a
// new file has not been created in its place, the fileOpener *may* create a
// new file, depending on its initial flags.  (spoiler alert: since we only use
// this once, and we do include the create flag, reopen will always create a
// file if none exists)
func (f *fileOpener) Reopen() error {
	f2, err := os.OpenFile(f.fname, f.flag, f.mode)
	if err != nil {
		return fmt.Errorf("fileOpener unable to reopen file: %v", err)
	}
	f1 := f.File
	f.File = f2
	return f1.Close() // hmm, do we care if we failed to close this file handle?
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

	c := make(chan []byte, options.lineBuf)
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
	flag.IntVar(&options.lineBuf, "linebuf", 200, "number of lines allowed to be in memory before writer is blocked")
}
