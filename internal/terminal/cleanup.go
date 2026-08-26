package terminal

import (
	"errors"
	"sync"
)

type OwnedResources struct {
	ClosePTY      func() error
	CloseClient   func() error
	ReleaseShadow func() error
}

type Cleanup struct {
	resources OwnedResources
	once      sync.Once
	err       error
}

func NewCleanup(resources OwnedResources) *Cleanup {
	if resources.ClosePTY == nil || resources.CloseClient == nil || resources.ReleaseShadow == nil {
		panic("terminal cleanup requires PTY, client, and shadow release functions") // justify-defect: the owning adapter must close all three published resources.
	}
	return &Cleanup{resources: resources}
}

func (cleanup *Cleanup) Close() error {
	cleanup.once.Do(func() {
		ptyError := cleanup.resources.ClosePTY()
		clientError := cleanup.resources.CloseClient()
		shadowError := cleanup.resources.ReleaseShadow()
		cleanup.err = errors.Join(ptyError, clientError, shadowError)
	})
	return cleanup.err
}
