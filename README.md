autosync
========

`autosync` is a utility to watch a set of sub-directories for file creates, copying the files to the Tailscale nodes named after those sub-directories using the `taildrop` facilities in tailscale.

The process is self-contained; it uses `fsnotify` to alert on file changes, and Tailscale itself to do the actual sending through an API call to the local Tailscale unix socket. 

As a result, `Tailscale` configured locally and running is a requirement for this to work.

There is no guarantee on this code, use at your own risk etc.

## Installation

`make install`

This assumes you will have an `~/autosync` directory, which subfolders corresponding to machines on your subnet.
