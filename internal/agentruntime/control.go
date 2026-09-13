package agentruntime

type Status struct {
	State  string `json:"state"`
	Source string `json:"source"`
	Reason string `json:"reason,omitempty"`
}

type Methods struct {
	Read      string `json:"read"`
	Send      string `json:"send"`
	Interrupt string `json:"interrupt"`
}

func (status Status) Valid() bool {
	switch status.State {
	case "working", "blocked", "idle", "done", "failed", "stopped", "unknown":
	default:
		return false
	}
	switch status.Source {
	case "native", "terminal", "unavailable":
	default:
		return false
	}
	switch status.Reason {
	case "", "permission", "input", "dialog", "provider_unavailable", "unrecognized":
		return true
	default:
		return false
	}
}

func (methods Methods) Valid() bool {
	for _, method := range []string{methods.Read, methods.Send, methods.Interrupt} {
		switch method {
		case "native", "terminal", "unavailable":
		default:
			return false
		}
	}
	return true
}
