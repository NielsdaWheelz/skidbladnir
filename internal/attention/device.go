package attention

import "slices"

// Admit merges producer facts without regressing independent local handling.
func Admit(device *DeviceSnapshot, producer Snapshot) (bool, error) {
	if device.ObserverEpoch != "" && device.ObserverEpoch != producer.Epoch {
		return false, ErrEpochChanged
	}
	if producer.Revision < device.AdmittedRevision {
		return false, ErrUnavailable
	}
	for _, host := range producer.Machines {
		if host.Availability != "fresh" {
			continue
		}
		for _, row := range host.Sessions {
			previous, found := device.Record(row.Record.Key)
			if found && SameForeground(previous.Foreground, row.Record.Foreground) && (row.Record.ReadyGeneration < previous.ReceivedReadyGeneration || row.Record.AttentionEpisode < previous.HandledAttentionEpisode) {
				return false, ErrUnavailable
			}
		}
	}
	changed := device.ObserverEpoch != producer.Epoch || device.AdmittedRevision != producer.Revision
	device.ObserverEpoch = producer.Epoch
	device.AdmittedRevision = producer.Revision
	for _, host := range producer.Machines {
		if host.Availability != "fresh" {
			continue
		}
		device.Records = slices.DeleteFunc(device.Records, func(record DeviceRecord) bool {
			if record.Key.Machine != host.Machine {
				return false
			}
			for _, row := range host.Sessions {
				if row.Record.Key.Slot() == record.Key.Slot() {
					return false
				}
			}
			changed = true
			return true
		})
		for _, row := range host.Sessions {
			source := row.Record
			index := slices.IndexFunc(device.Records, func(record DeviceRecord) bool { return record.Key.Slot() == source.Key.Slot() })
			record := DeviceRecord{Key: source.Key, Foreground: source.Foreground, Presentation: Closed}
			if index >= 0 {
				previous := device.Records[index]
				if previous.Key == source.Key && SameForeground(previous.Foreground, source.Foreground) {
					record = previous
				}
			}
			if source.ReadyGeneration > record.ReceivedReadyGeneration {
				record.ReceivedReadyGeneration = source.ReadyGeneration
			}
			if index < 0 {
				device.Records = append(device.Records, record)
				changed = true
				continue
			}
			previous := device.Records[index]
			if record.Key != previous.Key || !SameForeground(record.Foreground, previous.Foreground) || record.ReceivedReadyGeneration != previous.ReceivedReadyGeneration {
				device.Records[index] = record
				changed = true
			}
		}
	}
	return changed, nil
}

// Presented consumes only an exact identity's captured ready generation.
func Presented(device *DeviceSnapshot, key Key, foreground *Foreground, token ReadyToken) bool {
	if token.Epoch != device.ObserverEpoch || token.Generation == 0 {
		return false
	}
	for i := range device.Records {
		record := &device.Records[i]
		if record.Key != key || !SameForeground(record.Foreground, foreground) || token.Generation > record.ReceivedReadyGeneration || token.Generation <= record.AcknowledgedReadyGeneration {
			continue
		}
		record.AcknowledgedReadyGeneration = token.Generation
		return true
	}
	return false
}
