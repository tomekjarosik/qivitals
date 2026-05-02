package storage

import (
	"context"
	"testing"
)

func TestMemorySensorStorage_RegisterAndGet(t *testing.T) {
	storage := NewMemorySensorStorage()
	ctx := context.Background()

	sensor := &SensorInfo{
		ID:             "sensor-1",
		Name:           "db-backup",
		Description:    "Daily database backup",
		GracefulPeriod: 86400,
		FailurePeriod:  172800,
		Labels:         map[string]string{"env": "prod", "team": "data"},
	}

	// Test Registration
	err := storage.Register(ctx, sensor)
	if err != nil {
		t.Fatalf("Failed to register sensor: %v", err)
	}

	// Test duplicate ID
	err = storage.Register(ctx, sensor)
	if _, ok := err.(*DuplicateSensorError); !ok {
		t.Errorf("Expected DuplicateSensorError, got %v", err)
	}

	// Test duplicate Name
	sensorDiffID := &SensorInfo{ID: "sensor-2", Name: "db-backup"}
	err = storage.Register(ctx, sensorDiffID)
	if _, ok := err.(*DuplicateSensorError); !ok {
		t.Errorf("Expected DuplicateSensorError for duplicate name, got %v", err)
	}

	// Test GetStatus
	state, err := storage.GetStatus(ctx, "sensor-1")
	if err != nil {
		t.Fatalf("Failed to get status: %v", err)
	}

	if state.Info.Name != "db-backup" {
		t.Errorf("Expected name db-backup, got %s", state.Info.Name)
	}
	if state.Info.Labels["env"] != "prod" {
		t.Errorf("Expected env=prod label, got %s", state.Info.Labels["env"])
	}
}

func TestMemorySensorStorage_Update(t *testing.T) {
	storage := NewMemorySensorStorage()
	ctx := context.Background()

	storage.Register(ctx, &SensorInfo{ID: "s1", Name: "test", Description: "old"})

	updates := &SensorInfo{
		Name:        "new-test",
		Description: "new",
	}

	// Update only description
	err := storage.Update(ctx, "s1", updates, []string{"description"})
	if err != nil {
		t.Fatalf("Failed to update: %v", err)
	}

	state, _ := storage.GetStatus(ctx, "s1")
	if state.Info.Description != "new" {
		t.Errorf("Expected description to be 'new', got %s", state.Info.Description)
	}
	if state.Info.Name != "test" {
		t.Errorf("Expected name to remain 'test', got %s", state.Info.Name)
	}
}

func TestMemorySensorStorage_SendData(t *testing.T) {
	storage := NewMemorySensorStorage()
	ctx := context.Background()

	storage.Register(ctx, &SensorInfo{ID: "s1", Name: "test"})

	stateBefore, _ := storage.GetStatus(ctx, "s1")

	err := storage.SendData(ctx, "s1", true, map[string]string{"version": "1.2.3"})
	if err != nil {
		t.Fatalf("Failed to send data: %v", err)
	}

	stateAfter, _ := storage.GetStatus(ctx, "s1")

	if stateAfter.LastOkTimestamp < stateBefore.LastOkTimestamp {
		t.Errorf("LastOkTimestamp should have increased")
	}
	if stateAfter.Metadata["version"] != "1.2.3" {
		t.Errorf("Expected metadata version=1.2.3")
	}
}

func TestMemorySensorStorage_GetByNaturalKey(t *testing.T) {
	storage := NewMemorySensorStorage()
	ctx := context.Background()

	storage.Register(ctx, &SensorInfo{ID: "s1", Name: "my-job"})

	state, err := storage.GetByNaturalKey(ctx, "", "my-job")
	if err != nil {
		t.Fatalf("Failed to get by natural key: %v", err)
	}
	if state.Info.ID != "s1" {
		t.Errorf("Expected ID s1, got %s", state.Info.ID)
	}

	_, err = storage.GetByNaturalKey(ctx, "", "non-existent")
	if _, ok := err.(*SensorNotFoundError); !ok {
		t.Errorf("Expected SensorNotFoundError, got %v", err)
	}
}

func TestMemorySensorStorage_Query(t *testing.T) {
	storage := NewMemorySensorStorage()
	ctx := context.Background()

	storage.Register(ctx, &SensorInfo{ID: "s1", Name: "job-a", Labels: map[string]string{"env": "prod", "tier": "1"}})
	storage.Register(ctx, &SensorInfo{ID: "s2", Name: "job-b", Labels: map[string]string{"env": "prod", "tier": "2"}})
	storage.Register(ctx, &SensorInfo{ID: "s3", Name: "task-c", Labels: map[string]string{"env": "dev"}})

	tests := []struct {
		name          string
		filter        QueryFilter
		expectedCount int
	}{
		{
			name:          "Empty filter matches all",
			filter:        QueryFilter{},
			expectedCount: 3,
		},
		{
			name:          "Filter by ID",
			filter:        QueryFilter{ID: "s2"},
			expectedCount: 1,
		},
		{
			name:          "Filter by single label",
			filter:        QueryFilter{Labels: map[string]string{"env": "prod"}},
			expectedCount: 2,
		},
		{
			name:          "Filter by multiple labels",
			filter:        QueryFilter{Labels: map[string]string{"env": "prod", "tier": "1"}},
			expectedCount: 1,
		},
		{
			name:          "Filter by non-matching label",
			filter:        QueryFilter{Labels: map[string]string{"env": "prod", "tier": "3"}},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := storage.Query(ctx, tt.filter)
			if err != nil {
				t.Fatalf("Query failed: %v", err)
			}
			if len(results) != tt.expectedCount {
				t.Errorf("Expected %d results, got %d", tt.expectedCount, len(results))
			}
		})
	}
}
