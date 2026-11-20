package fleetclient

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/claudeusage"
	"github.com/NielsdaWheelz/skidbladnir/internal/privatefile"
)

// A profile name belongs to the fleet configuration. Its Claude login can
// change on one machine independently of that login's measured quota.
type claudeBinding struct {
	Profile
	Account string `json:"account,omitempty"`
}

type claudeBindings struct {
	ObservedAt time.Time       `json:"observedAt"`
	Profiles   []claudeBinding `json:"profiles"`
}

// The open browser can keep a reading even when its local disk is unwritable.
type claudeHistory struct {
	mu       sync.Mutex
	machines map[string]claudeBindings
	accounts map[string]api.ClaudeUsage
}

func (peer ProfileUsagePeer) claudeReadings() []api.ClaudeUsage {
	readings := slices.Clone(peer.OtherClaudeAccounts)
	for _, row := range peer.Profiles {
		if row.Claude != nil {
			readings = append(readings, *row.Claude)
		}
	}
	return readings
}

func (client *Client) bindingPath(machine string) string {
	return filepath.Join(client.directory, "claude-usage", "machines", machine+".json")
}

func readClaudeBindings(path string) (claudeBindings, error) {
	var saved claudeBindings
	data, err := privatefile.Read(path, MaximumControlBytes)
	if errors.Is(err, os.ErrNotExist) {
		return saved, nil
	}
	if err != nil {
		return saved, err
	}
	err = json.Unmarshal(data, &saved)
	return saved, err
}

func (peer ProfileUsagePeer) claudeBindings(previous []claudeBinding) []claudeBinding {
	var bindings []claudeBinding
	for _, row := range peer.Profiles {
		if row.Provider != "Claude" {
			continue
		}
		binding := claudeBinding{Profile: row.Profile}
		if row.Claude != nil {
			binding.Account = row.Claude.Account
		}
		if binding.Account == "" {
			// An unreadable cache supplies no new login for this profile home.
			for _, old := range previous {
				if old.Key == binding.Key && old.AccountScope == binding.AccountScope {
					binding.Account = old.Account
					break
				}
			}
		}
		bindings = append(bindings, binding)
	}
	return bindings
}

// Both the browser and the background gateway can be first to see a reading.
func (client *Client) rememberClaudePeer(peer ProfileUsagePeer) (err error) {
	for _, reading := range peer.claudeReadings() {
		_, saveErr := claudeusage.Remember(client.directory, reading)
		err = errors.Join(err, saveErr)
	}
	path := client.bindingPath(peer.Machine)
	lock, lockErr := privatefile.Lock(path + ".lock")
	if lockErr != nil {
		return errors.Join(err, lockErr)
	}
	defer func() { err = errors.Join(err, lock.Close()) }()
	previous, readErr := readClaudeBindings(path)
	if readErr != nil || !peer.ObservedAt.After(previous.ObservedAt) {
		return errors.Join(err, readErr)
	}
	bindings := peer.claudeBindings(previous.Profiles)
	if len(bindings) == 0 && len(previous.Profiles) == 0 {
		return err
	}
	data, encErr := json.Marshal(claudeBindings{peer.ObservedAt, bindings})
	if encErr != nil {
		return errors.Join(err, encErr)
	}
	return errors.Join(err, privatefile.WriteFile(path, data))
}

// KnownClaudeUsage works with every gateway offline. The local login resolves
// a profile key. Without one, its enrolled sources must agree on the account.
func (client *Client) KnownClaudeUsage(recent ...ProfileUsagePeer) []ProfileUsage {
	client.mu.RLock()
	self := client.self.Machine
	peers := append([]peer(nil), client.peers...)
	client.mu.RUnlock()

	history := &client.claudeHistory
	history.mu.Lock()
	defer history.mu.Unlock()
	if history.machines == nil {
		history.machines = make(map[string]claudeBindings)
		history.accounts = make(map[string]api.ClaudeUsage)
	}

	byMachine := make(map[string]ProfileUsagePeer)
	for _, response := range recent {
		if !response.OK {
			continue
		}
		byMachine[response.Machine] = response
		for _, observed := range response.claudeReadings() {
			r := history.accounts[observed.Account]
			r.Merge(observed)
			history.accounts[observed.Account] = r
		}
	}
	type choice struct {
		claudeBinding
		local, ambiguous bool
	}
	byKey := make(map[string]*choice)
	var ordered []*choice
	for _, source := range peers {
		bindings := history.machines[source.Machine]
		if saved, err := readClaudeBindings(client.bindingPath(source.Machine)); err == nil && saved.ObservedAt.After(bindings.ObservedAt) {
			bindings = saved
		}
		if response, ok := byMachine[source.Machine]; ok && response.ObservedAt.After(bindings.ObservedAt) {
			bindings = claudeBindings{response.ObservedAt, response.claudeBindings(bindings.Profiles)}
		}
		history.machines[source.Machine] = bindings
		for _, binding := range bindings.Profiles {
			current := byKey[binding.Key]
			if current == nil {
				current = &choice{claudeBinding: claudeBinding{Profile: binding.Profile}}
				byKey[binding.Key] = current
				ordered = append(ordered, current)
			}
			if source.Machine == self {
				current.Profile = binding.Profile
			}
			if current.local || binding.Account == "" {
				continue
			}
			switch {
			case source.Machine == self:
				current.Account, current.local, current.ambiguous = binding.Account, true, false
			case current.Account == "":
				current.Account = binding.Account
			case current.Account != binding.Account:
				current.ambiguous = true
			}
		}
	}
	rows := make([]ProfileUsage, 0, len(ordered))
	for _, chosen := range ordered {
		row := ProfileUsage{Profile: chosen.Profile, ReadState: "unavailable"}
		if chosen.Account != "" && !chosen.ambiguous {
			record := history.accounts[chosen.Account]
			if saved, err := claudeusage.Load(client.directory, chosen.Account); err == nil {
				record.Merge(saved)
			}
			history.accounts[chosen.Account] = record
			if record.HasReading() {
				row.Claude = &record
				row.ReadState = "ok"
			}
		}
		rows = append(rows, row)
	}
	return rows
}
