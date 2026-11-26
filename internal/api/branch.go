package api

// SessionRef identifies a session across devices and changes to its selected pane.
type SessionRef struct {
	Machine       string `json:"machine"`
	TmuxID        string `json:"tmuxId"`
	IdentityToken string `json:"identityToken"`
}

type BranchNode struct {
	Target   Target       `json:"target"`
	Name     string       `json:"name"`
	Parent   *SessionRef  `json:"parent,omitempty"`
	Previous []SessionRef `json:"previous,omitempty"`
	Children []SessionRef `json:"children"`
	Pending  bool         `json:"pending"`
}

type BranchRequest struct {
	Action        string      `json:"action"`
	IdentityToken string      `json:"identityToken"`
	ID            string      `json:"id,omitempty"`
	Machine       string      `json:"machine,omitempty"`
	CloseID       string      `json:"closeId,omitempty"`
	Mode          string      `json:"mode,omitempty"`
	Parent        *SessionRef `json:"parent,omitempty"`
	Child         *SessionRef `json:"child,omitempty"`
	Expected      *BranchNode `json:"expected,omitempty"`
}

type BranchReservation struct {
	ID string `json:"id"`
}
