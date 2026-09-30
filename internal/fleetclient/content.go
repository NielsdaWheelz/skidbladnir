package fleetclient

func SessionStatusDetail(session Session) string {
	text := SessionStatus(session)
	if session.Agent != nil && session.TerminalStatus.Source == "terminal" {
		text += " · inferred from terminal"
	}
	return text
}

// ErrorMessage is shared by command output and the desktop browser.
func ErrorMessage(failure Failure, request Request, native bool) string {
	if failure.Dispatch == "unknown" {
		if request.Operation == "rename" {
			return "name change outcome unknown. checking tmux."
		}
		if request.Operation == "group" {
			return "group change outcome unknown. checking tmux."
		}
		if request.Operation == "start" || request.Operation == "shell" {
			return "could not confirm terminal creation. refresh before taking another action."
		}
		if native {
			return "could not confirm the native request. inspect the conversation before trying again."
		}
		if request.Operation == "close" {
			if request.TerminalOnly {
				return "could not confirm terminal deletion. refresh before taking another action."
			}
			return "could not confirm terminal interruption or deletion. inspect the terminal if available and refresh before taking another action."
		}
		return "could not confirm terminal input. inspect the terminal before trying again."
	}
	switch failure.Code {
	case "SessionNameInvalid":
		return "use 1–64 letters, numbers, underscores, or hyphens; start with a letter or number."
	case "SessionNameConflict":
		return "another session on this machine uses that name."
	case "SessionNameChanged":
		return "the session name changed. review and save again."
	case "TerminalTargetChanged", "SessionIdentityMismatch", "SessionNotFound":
		return "the terminal changed. refresh before trying again."
	case "TerminalInputBlocked":
		if failure.terminalInputMessage != "" {
			return failure.terminalInputMessage
		}
		return "send unavailable for this screen. open the terminal or use text/keys."
	case "TerminalUnavailable":
		return "terminal action unavailable. open the terminal or refresh before trying again."
	case "AgentTargetStale":
		return "the native conversation changed. inspect it before trying again."
	case "AgentUnavailable":
		return "this native action is unavailable for the selected conversation."
	default:
		return failure.Code + " (" + failure.Dispatch + ")"
	}
}

func WriteText(operation string, receipt WriteResult) string {
	if receipt.Outcome == "unknown" {
		if receipt.Method == "native" {
			return "could not confirm native interruption. inspect the conversation before trying again."
		}
		return "could not confirm terminal input. inspect the terminal before trying again."
	}
	if receipt.Method == "native" {
		return "native work: " + receipt.Outcome + "; pending input may remain."
	}
	switch operation {
	case "stop":
		return "interrupt key sent; stopping is unconfirmed."
	case "text", "send":
		return "text sent; acceptance is unconfirmed."
	default:
		return "keys sent; acceptance is unconfirmed."
	}
}

func CloseText(receipt CloseResult) string {
	if receipt.Terminal == "closed" {
		if receipt.Interrupt != "written" {
			return "terminal closed; interruption unconfirmed."
		}
		return "interrupt key sent; terminal closed. work shared elsewhere or running remotely may continue."
	}
	return "interruption: " + receipt.Interrupt + "; terminal: " + receipt.Terminal + ". work shared elsewhere or running remotely may continue."
}
