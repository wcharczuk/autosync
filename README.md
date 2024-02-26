autosync
========

`autosync` is a utility to watch a set of sub-directories for file creates, copying the files to the Tailscale nodes named after those sub-directories using the `taildrop` facilities in tailscale.

The process is self-contained; it uses `fsnotify` to alert on file changes, and the "guts" of Tailscale's CLI code to do the actual sending.

There is no guarantee on this code, use at your own risk etc.
