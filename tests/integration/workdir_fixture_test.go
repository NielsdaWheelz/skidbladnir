//go:build integration

package integration_test

import (
	"testing"

	"github.com/NielsdaWheelz/skidbladnir/internal/workdir"
)

func newWorkdirFixture(t *testing.T, home string) *workdir.Service {
	t.Helper()
	service, err := workdir.New(home)
	if err != nil {
		t.Fatal("construct integration working directory service")
	}
	return service
}
