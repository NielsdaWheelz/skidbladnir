package terminal

import (
	"errors"
	"sync"
)

type OwnedResources struct {
	ClosePTY    func() error
	CloseClient func() error
}

type Cleanup struct {
	resources OwnedResources
	once      sync.Once
	err       error
}

func NewCleanup(resources OwnedResources) *Cleanup {
	if resources.ClosePTY == nil || resources.CloseClient == nil {
		panic("terminal cleanup requires PTY and client close functions") // justify-defect: the owning adapter must close both published resources.
	}
	return &Cleanup{resources: resources}
}

func (cleanup *Cleanup) Close() error {
	cleanup.once.Do(func() {
		ptyError := cleanup.resources.ClosePTY()
		clientError := cleanup.resources.CloseClient()
		cleanup.err = errors.Join(ptyError, clientError)
	})
	return cleanup.err
}
