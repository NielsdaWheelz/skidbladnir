package fleetclient

import "regexp"

// Naming describes ownership of the actual tmux session name.
type Naming struct {
	Mode string `json:"mode"`
	Name string `json:"name,omitempty"`
}

func (session Session) Naming() Naming {
	if session.NameMode == "automatic" {
		return Naming{Mode: "automatic"}
	}
	return Naming{Mode: session.NameMode, Name: session.Name}
}

func (naming Naming) valid() bool {
	return naming.Mode == "automatic" && naming.Name == "" || naming.Mode == "manual" && naming.Name != ""
}

var sessionNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

func ValidSessionName(name string) bool { return sessionNamePattern.MatchString(name) }
