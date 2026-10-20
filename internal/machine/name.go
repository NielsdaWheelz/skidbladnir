package machine

import (
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

func ValidName(name string) bool {
	return name != "" && utf8.ValidString(name) && strings.TrimSpace(name) == name && strings.IndexFunc(name, unicode.IsControl) < 0
}
func ValidOrigin(origin string) bool {
	u, e := url.Parse(origin)
	if e != nil {
		return false
	}
	host := u.Hostname()
	if host != strings.ToLower(host) || u.Host != host+":8443" || len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if c != '-' && (c < 'a' || c > 'z') && (c < '0' || c > '9') {
				return false
			}
		}
	}
	return u.Scheme == "https" && strings.HasSuffix(host, ".ts.net") && host != ".ts.net" && u.User == nil && u.Path == "" && u.RawQuery == "" && !u.ForceQuery && u.Fragment == "" && u.Opaque == "" && u.String() == origin
}
