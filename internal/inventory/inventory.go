package inventory

import "sort"

// PaneFact is the validated tmux and terminal identity needed to classify one
// pane. The caller owns collection and validation of these boundary facts.
type PaneFact struct {
	ID                     string
	RootPID                int
	TTY                    string
	ForegroundProcessGroup int
}

// ProcessFact is the validated /proc identity needed by the unmarked-TUI
// predicate. RuntimeMarked records the presence of a runtime marker; its value
// is intentionally outside this classifier.
type ProcessFact struct {
	PID           int
	ParentPID     int
	ProcessGroup  int
	TTY           string
	Executable    string
	ProfileHome   string
	RuntimeMarked bool
	StartTime     uint64
}

// Candidate is a proven attach identity. Registry, card, and persistence
// projection are deliberately outside this package.
type Candidate struct {
	PaneID      string
	PID         int
	StartTime   uint64
	TTY         string
	ProfileHome string
}

// ClassifyUnmarkedTUIs applies the fail-closed unmarked-TUI predicate to
// caller-supplied tmux and /proc facts. It performs no boundary I/O.
func ClassifyUnmarkedTUIs(panes []PaneFact, processes []ProcessFact, pinnedExecutable string, recognizedProfileHomes []string) []Candidate {
	if pinnedExecutable == "" {
		return nil
	}
	recognizedHomes := make(map[string]struct{}, len(recognizedProfileHomes))
	for _, home := range recognizedProfileHomes {
		if home != "" {
			recognizedHomes[home] = struct{}{}
		}
	}

	byPID := make(map[int]ProcessFact, len(processes))
	duplicatePIDs := make(map[int]struct{})
	for _, process := range processes {
		if process.PID <= 0 {
			continue
		}
		if _, exists := byPID[process.PID]; exists {
			duplicatePIDs[process.PID] = struct{}{}
			continue
		}
		byPID[process.PID] = process
	}

	var candidates []Candidate
	for _, pane := range panes {
		if pane.ID == "" || pane.RootPID <= 0 || pane.TTY == "" || pane.ForegroundProcessGroup <= 0 {
			continue
		}
		structural := make(map[int]ProcessFact)
		for _, process := range processes {
			if !eligible(process, pane, pinnedExecutable, recognizedHomes) {
				continue
			}
			if _, duplicate := duplicatePIDs[process.PID]; duplicate {
				continue
			}
			paneRoot := process.PID == pane.RootPID
			foregroundLeader := process.PID == process.ProcessGroup && process.ProcessGroup == pane.ForegroundProcessGroup
			if paneRoot || foregroundLeader {
				structural[process.PID] = process
			}
		}
		if len(structural) != 1 {
			continue
		}
		for _, process := range structural {
			if !provenNonNestedAncestry(process, byPID, duplicatePIDs, pinnedExecutable) {
				continue
			}
			candidates = append(candidates, Candidate{
				PaneID:      pane.ID,
				PID:         process.PID,
				StartTime:   process.StartTime,
				TTY:         process.TTY,
				ProfileHome: process.ProfileHome,
			})
		}
	}

	sort.Slice(candidates, func(left, right int) bool {
		if candidates[left].PaneID != candidates[right].PaneID {
			return candidates[left].PaneID < candidates[right].PaneID
		}
		return candidates[left].PID < candidates[right].PID
	})
	return candidates
}

func eligible(process ProcessFact, pane PaneFact, pinnedExecutable string, recognizedHomes map[string]struct{}) bool {
	if process.PID <= 0 || process.ParentPID < 0 || process.ProcessGroup <= 0 || process.StartTime == 0 {
		return false
	}
	if process.TTY != pane.TTY || process.Executable != pinnedExecutable || process.RuntimeMarked {
		return false
	}
	_, recognized := recognizedHomes[process.ProfileHome]
	return recognized
}

func provenNonNestedAncestry(process ProcessFact, byPID map[int]ProcessFact, duplicatePIDs map[int]struct{}, pinnedExecutable string) bool {
	if process.ParentPID == 0 {
		return process.PID == 1
	}
	seen := map[int]struct{}{process.PID: {}}
	parent := process.ParentPID
	for parent > 0 {
		if _, cycle := seen[parent]; cycle {
			return false
		}
		seen[parent] = struct{}{}
		if _, duplicate := duplicatePIDs[parent]; duplicate {
			return false
		}
		ancestor, exists := byPID[parent]
		if !exists || ancestor.ParentPID < 0 || ancestor.StartTime == 0 {
			return false
		}
		if ancestor.Executable == pinnedExecutable {
			return false
		}
		if ancestor.PID == 1 && ancestor.ParentPID == 0 {
			return true
		}
		if ancestor.ParentPID == 0 {
			return false
		}
		if ancestor.Executable == "" {
			return false
		}
		parent = ancestor.ParentPID
	}
	return false
}
