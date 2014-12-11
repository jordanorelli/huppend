# huppend

huppend reads data on stdin and appends it to a specified file, `$outfile`.
Additionally, it listens for SIGHUP.  When huppend receives SIGHUP, it closes
its handle on `$outfile` and opens a new handle.  This is useful for log
rotation.

## usage

`huppend --pidfile=huppend.pid outfile.log`
