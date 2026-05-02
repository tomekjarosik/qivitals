package storage

import (
	"testing"
)

func TestMemoryStorageContract(t *testing.T) {
	setup := func() SensorStorage {
		return NewMemorySensorStorage()
	}

	teardown := func() {
		// Memory storage is garbage collected, no cleanup needed
	}

	RunStorageContractTests(t, setup, teardown)
	RunExtendedStorageContractTests(t, setup, teardown)
}
