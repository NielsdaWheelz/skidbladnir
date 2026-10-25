package tmux

// ProtocolError identifies rejected native output without retaining its values.
// Record is batch_begin, batch_end, inventory_row, session_metadata,
// workspace_session, workspace_window, workspace_pane, workspace_graph,
// server_preflight or guard_completion. Field is 1-based, or zero for the record.
// Reason is framing, invalid_value, duplicate, inconsistent or unexpected_marker.
type ProtocolError struct {
	Record string
	Field  int
	Reason string
}

func (failure *ProtocolError) Error() string { return WorkspaceProtocolInvalid.Error() }
func (failure *ProtocolError) Unwrap() error { return WorkspaceProtocolInvalid }
