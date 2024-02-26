package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/urfave/cli/v2"

	"golang.org/x/net/idna"
	"tailscale.com/client/tailscale"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/net/tsaddr"
	"tailscale.com/paths"
	"tailscale.com/tailcfg"
	"tailscale.com/util/dnsname"
)

func main() {
	app := &cli.App{
		Name:  "autosync",
		Usage: "syncronize files to tailscale targets based on directories",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "source",
				Usage: "the directory to watch for changes",
			},
			&cli.BoolFlag{
				Name:  "rm",
				Usage: "if we should remove files once they're done copying",
			},
		},
		Action: start,
	}
	localClient.Socket = paths.DefaultTailscaledSocket()
	if err := app.Run(os.Args); err != nil {
		slog.Error("run error: %v", err)
		os.Exit(1)
	}
}

func start(ctx *cli.Context) error {
	sourceDir := ctx.String("source")
	if sourceDir == "" {
		pwd, err := os.Getwd()
		if err != nil {
			return err
		}
		sourceDir = pwd
	} else {
		sourceDir = os.ExpandEnv(sourceDir)
	}

	dirEntries, err := os.ReadDir(sourceDir)
	if err != nil {
		return err
	}
	for _, dir := range dirEntries {
		if !dir.IsDir() {
			continue
		}
		_, err := watchDir(ctx, filepath.Join(sourceDir, dir.Name()), dir.Name()+":")
		if err != nil {
			return err
		}
	}
	select {}
}

func fileAllowCopy(filename string) bool {
	if filepath.Base(filename) == ".DS_Store" {
		return false
	}
	return true
}

func watchDir(ctx *cli.Context, watchDir, targetServer string) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	go func() {
		slog.Info("eventloop starting")
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					slog.Info("eventloop exiting")
					return
				}
				if event.Has(fsnotify.Create) {
					slog.Info("filevent", "event", event)
					if fileAllowCopy(event.Name) {
						go tsCopyFilesAsync(ctx.Context, []string{event.Name}, targetServer, ctx.Bool("rm") /*remove files*/)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					slog.Info("eventloop exiting")
					return
				}
				slog.Error("watcher error", "err", err)
			}
		}
	}()

	slog.Info("watching", "dir", watchDir)
	err = watcher.Add(watchDir)
	if err != nil {
		return watcher, err
	}
	return watcher, nil
}

var (
	localClient tailscale.LocalClient // in-memory
)

func tsCopyFilesAsync(ctx context.Context, files []string, target string, removeOnComplete bool) {
	start := time.Now()
	slog.Info("tailscale copy files", "files", files, "target", target)
	defer func() {
		slog.Info("tailscale copy files complete", "files", files, "target", target, "elapsed", time.Since(start).Round(time.Millisecond))
	}()

	if err := tsCopyFiles(ctx, files, target, removeOnComplete); err != nil {
		slog.Error("tailscale copy error", "err", err)
	}
}

func tsCopyFiles(ctx context.Context, files []string, target string, removeOnComplete bool) error {
	target, ok := strings.CutSuffix(target, ":")
	if !ok {
		return fmt.Errorf("final argument to 'copyFile' must end in colon")
	}
	hadBrackets := false
	if strings.HasPrefix(target, "[") && strings.HasSuffix(target, "]") {
		hadBrackets = true
		target = strings.TrimSuffix(strings.TrimPrefix(target, "["), "]")
	}
	if ip, err := netip.ParseAddr(target); err == nil && ip.Is6() && !hadBrackets {
		return fmt.Errorf("an IPv6 literal must be written as [%s]", ip)
	} else if hadBrackets && (err != nil || !ip.Is6()) {
		return errors.New("unexpected brackets around target")
	}
	ip, _, err := tailscaleIPFromArg(ctx, target)
	if err != nil {
		return err
	}

	stableID, isOffline, err := getTargetStableID(ctx, ip)
	if err != nil {
		return fmt.Errorf("can't send to %s: %v", target, err)
	}
	if isOffline {
		slog.Error("tailscale: warning host is offline", "target", target)
	}

	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()
		fi, err := f.Stat()
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return errors.New("directories not supported")
		}
		fileArg := filepath.Base(file)
		contentLength := fi.Size()
		err = localClient.PushFile(ctx, stableID, contentLength, fileArg, f)
		if err != nil {
			return err
		}
		if removeOnComplete {
			if err = os.Remove(file); err != nil {
				return err
			}
		}
	}
	return nil
}

func tailscaleIPFromArg(ctx context.Context, hostOrIP string) (ip string, self bool, err error) {
	// If the argument is an IP address, use it directly without any resolution.
	if net.ParseIP(hostOrIP) != nil {
		return hostOrIP, false, nil
	}

	// Otherwise, try to resolve it first from the network peer list.
	st, err := localClient.Status(ctx)
	if err != nil {
		return "", false, err
	}
	match := func(ps *ipnstate.PeerStatus) bool {
		return strings.EqualFold(hostOrIP, dnsOrQuoteHostname(st, ps)) || hostOrIP == ps.DNSName
	}
	for _, ps := range st.Peer {
		if match(ps) {
			if len(ps.TailscaleIPs) == 0 {
				return "", false, errors.New("node found but lacks an IP")
			}
			return ps.TailscaleIPs[0].String(), false, nil
		}
	}
	if match(st.Self) && len(st.Self.TailscaleIPs) > 0 {
		return st.Self.TailscaleIPs[0].String(), true, nil
	}

	// Finally, use DNS.
	var res net.Resolver
	if addrs, err := res.LookupHost(ctx, hostOrIP); err != nil {
		return "", false, fmt.Errorf("error looking up IP of %q: %v", hostOrIP, err)
	} else if len(addrs) == 0 {
		return "", false, fmt.Errorf("no IPs found for %q", hostOrIP)
	} else {
		return addrs[0], false, nil
	}
}

func dnsOrQuoteHostname(st *ipnstate.Status, ps *ipnstate.PeerStatus) string {
	baseName := dnsname.TrimSuffix(ps.DNSName, st.MagicDNSSuffix)
	if baseName != "" {
		if strings.HasPrefix(baseName, "xn-") {
			if u, err := idna.ToUnicode(baseName); err == nil {
				return fmt.Sprintf("%s (%s)", baseName, u)
			}
		}
		return baseName
	}
	return fmt.Sprintf("(%q)", dnsname.SanitizeHostname(ps.HostName))
}

func getTargetStableID(ctx context.Context, ipStr string) (id tailcfg.StableNodeID, isOffline bool, err error) {
	ip, err := netip.ParseAddr(ipStr)
	if err != nil {
		return "", false, err
	}
	fts, err := localClient.FileTargets(ctx)
	if err != nil {
		return "", false, err
	}
	for _, ft := range fts {
		n := ft.Node
		for _, a := range n.Addresses {
			if a.Addr() != ip {
				continue
			}
			isOffline = n.Online != nil && !*n.Online
			return n.StableID, isOffline, nil
		}
	}
	return "", false, fileTargetErrorDetail(ctx, ip)
}

func fileTargetErrorDetail(ctx context.Context, ip netip.Addr) error {
	found := false
	if st, err := localClient.Status(ctx); err == nil && st.Self != nil {
		for _, peer := range st.Peer {
			for _, pip := range peer.TailscaleIPs {
				if pip == ip {
					found = true
					if peer.UserID != st.Self.UserID {
						return errors.New("owned by different user; can only send files to your own devices")
					}
				}
			}
		}
	}
	if found {
		return errors.New("target seems to be running an old Tailscale version")
	}
	if !tsaddr.IsTailscaleIP(ip) {
		return fmt.Errorf("unknown target; %v is not a Tailscale IP address", ip)
	}
	return errors.New("unknown target; not in your Tailnet")
}
