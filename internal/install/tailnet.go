package install

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var errTailnetRouteAbsent = errors.New("recorded tailscale route is absent")

type tailnetRoute struct {
	Origin string `json:"origin"`
	Target string `json:"target"`
}

// ConfigureTailnetRoute owns Serve's exact https:8443 proxy and its removal receipt.
// The caller must first authenticate the loopback gateway's expected machine and
// release identity. File ownership alone cannot identify a process bound to a port.
func ConfigureTailnetRoute(ctx context.Context, origin, target string) (result error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	parsed, e := url.Parse(origin)
	if e != nil || parsed.Scheme != "https" || parsed.Port() != "8443" || parsed.Path != "" || parsed.RawQuery != "" || parsed.User != nil {
		return errors.New("installed tailnet origin must be https on port 8443")
	}
	proxy, e := url.Parse(target)
	if e != nil || proxy.Scheme != "http" || proxy.Hostname() != "127.0.0.1" || proxy.Port() == "" || proxy.Path != "" || proxy.RawQuery != "" || proxy.User != nil {
		return errors.New("installed tailnet proxy must be an http loopback address")
	}
	dirs, e := Paths()
	if e != nil {
		return e
	}
	lock, e := lockInstallation(dirs)
	if e != nil {
		return e
	}
	defer func() { result = errors.Join(result, lock.Close()) }()
	i := installer{dirs: dirs, stdout: io.Discard, stderr: io.Discard}
	data, e := os.ReadFile(filepath.Join(dirs.State, "install-owned.json"))
	if e != nil {
		return errors.New("install skid before connecting tailscale")
	}
	if e = decodeReceipt(data, &i.receipt); e != nil {
		return e
	}
	if i.receipt.Version == "" || len(i.receipt.Services) == 0 {
		return errors.New("install skid services before connecting tailscale")
	}
	if e = i.checkOwned(); e != nil {
		return e
	}
	if _, e = os.Stat(filepath.Join(dirs.Data, "current", "skidbladnir")); e != nil {
		return errors.New("installed skid release is unavailable")
	}
	route := tailnetRoute{origin, target}
	if i.receipt.Route == nil {
		e = matchTailnetRoute(ctx, route)
		if !errors.Is(e, errTailnetRouteAbsent) {
			if e != nil {
				return e
			}
			return errors.New("tailscale port 8443 is not installer-owned; preserve it and resolve manually")
		}
	} else {
		e = matchTailnetRoute(ctx, *i.receipt.Route)
		if e != nil && !errors.Is(e, errTailnetRouteAbsent) {
			return e
		}
		if e == nil && *i.receipt.Route != route {
			return errors.New("owned tailscale route differs; remove the recorded route before reconnecting")
		}
	}
	// Persist ownership before the external effect: uncertain command completion
	// may create the route, and uninstall must still know the exact admitted tuple.
	i.receipt.Route = &route
	if e = i.save(); e != nil {
		return e
	}
	tailscale, e := TailscalePath()
	if e != nil {
		return e
	}
	if e = command(ctx, io.Discard, io.Discard, tailscale, "serve", "--bg", "--https=8443", target); e != nil {
		return fmt.Errorf("tailscale route configuration failed; exact ownership retained: %w", e)
	}
	return matchTailnetRoute(ctx, route)
}

func matchTailnetRoute(ctx context.Context, route tailnetRoute) error {
	tailscale, e := TailscalePath()
	if e != nil {
		return e
	}
	output, e := exec.CommandContext(ctx, tailscale, "serve", "status", "--json").Output()
	if e != nil {
		return errors.New("cannot inspect owned tailscale route; nothing removed")
	}
	var status struct {
		TCP map[string]struct {
			HTTPS bool `json:"HTTPS"`
		} `json:"TCP"`
		Web map[string]struct {
			Handlers map[string]struct {
				Proxy string `json:"Proxy"`
			} `json:"Handlers"`
		} `json:"Web"`
		AllowFunnel map[string]bool `json:"AllowFunnel"`
	}
	if e = json.Unmarshal(output, &status); e != nil {
		return errors.New("cannot parse tailscale route ownership")
	}
	origin, _ := url.Parse(route.Origin)
	host := origin.Host
	_, hasTCP := status.TCP["8443"]
	hasWeb := false
	for key := range status.Web {
		if strings.HasSuffix(key, ":8443") {
			hasWeb = true
			break
		}
	}
	if !hasTCP && !hasWeb {
		return errTailnetRouteAbsent
	}
	web, ok := status.Web[host]
	if !ok || len(web.Handlers) != 1 || web.Handlers["/"].Proxy != route.Target || !status.TCP["8443"].HTTPS || status.AllowFunnel[host] {
		return errors.New("tailscale route changed; preserve it and resolve port 8443 manually")
	}
	for key := range status.Web {
		if key != host && strings.HasSuffix(key, ":8443") {
			return errors.New("tailscale port 8443 has additional routes; preserve them and resolve manually")
		}
	}
	return nil
}
func (i *installer) removeTailnetRoute(ctx context.Context) error {
	if i.receipt.Route == nil {
		return nil
	}
	if e := matchTailnetRoute(ctx, *i.receipt.Route); errors.Is(e, errTailnetRouteAbsent) {
		i.receipt.Route = nil
		return i.save()
	} else if e != nil {
		return e
	}
	tailscale, e := TailscalePath()
	if e != nil {
		return e
	}
	if e := command(ctx, i.stdout, i.stderr, tailscale, "serve", "--https=8443", "off"); e != nil {
		return fmt.Errorf("remove recorded tailscale route: %w", e)
	}
	i.receipt.Route = nil
	return i.save()
}
