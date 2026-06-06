package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// SnapshotStorage wraps an existing SensorStorage (like MemoryStorage)
// and periodically persists its state to a JSON file.
type SnapshotStorage struct {
	underlying SensorStorage
	filePath   string
	interval   time.Duration
	stopChan   chan struct{}
	wg         sync.WaitGroup
}

// NewSnapshotStorage wraps the provided storage and starts the snapshot loop.
func NewSnapshotStorage(underlying SensorStorage, filePath string, interval time.Duration) *SnapshotStorage {
	s := &SnapshotStorage{
		underlying: underlying,
		filePath:   filePath,
		interval:   interval,
		stopChan:   make(chan struct{}),
	}

	// 1. Try to load from disk on startup to "prime" the underlying storage
	// Note: This assumes your underlying storage has a way to be seeded,
	// or we simply load it and manually inject it back into the underlying storage.
	s.loadInitialState()

	// 2. Start the background periodic saver
	s.wg.Add(1)
	go s.runSnapshotLoop()

	return s
}

// Close stops the snapshot loop and performs a final save.
func (s *SnapshotStorage) Close() error {
	close(s.stopChan)
	s.wg.Wait()
	return s.takeSnapshot()
}

func (s *SnapshotStorage) runSnapshotLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.takeSnapshot(); err != nil {
				fmt.Printf("Snapshot error: %v\n", err)
			}
		case <-s.stopChan:
			return
		}
	}
}

func (s *SnapshotStorage) takeSnapshot() error {
	ctx := context.Background()
	states, err := s.underlying.Query(ctx, QueryFilter{})
	if err != nil {
		return err
	}

	dataMap := make(map[string]*SensorState)
	for _, state := range states {
		if state.Info != nil {
			dataMap[state.Info.ID] = state
		}
	}

	newBytes, err := json.MarshalIndent(dataMap, "", "  ")
	if err != nil {
		return err
	}

	// Avoid unnecessary disk writes if the content hasn't changed
	existingBytes, err := os.ReadFile(s.filePath)
	if err == nil && bytes.Equal(newBytes, existingBytes) {
		return nil
	}

	// Atomic write: write to temp file first, then rename
	tmpFile := s.filePath + ".tmp"
	if err := os.WriteFile(tmpFile, newBytes, 0644); err != nil {
		return err
	}

	return os.Rename(tmpFile, s.filePath)
}

func (s *SnapshotStorage) loadInitialState() {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return // File doesn't exist, nothing to load
	}

	var loadedStates map[string]*SensorState
	if err := json.Unmarshal(data, &loadedStates); err != nil {
		fmt.Printf("Failed to parse snapshot: %v\n", err)
		return
	}

	// Re-inject the loaded data into the underlying memory storage
	for _, state := range loadedStates {
		if state.Info != nil {
			// We call Register to put it back into the memory storage
			// We assume Register handles the logic of "if exists, skip"
			_ = s.underlying.Register(context.Background(), state.Info)

			// If you have a way to restore the Metadata/LastUpdated,
			// you'd call a Patch or custom method here.
		}
	}
}

// --- Passthrough Methods ---
// These methods simply delegate all work to the underlying storage.

func (s *SnapshotStorage) Register(ctx context.Context, sensor *SensorInfo) error {
	return s.underlying.Register(ctx, sensor)
}

func (s *SnapshotStorage) Delete(ctx context.Context, sensorID string) error {
	return s.underlying.Delete(ctx, sensorID)
}

func (s *SnapshotStorage) Patch(ctx context.Context, sensorID string, expectedVersion string, updates *SensorInfo, columns []string) error {
	return s.underlying.Patch(ctx, sensorID, expectedVersion, updates, columns)
}

func (s *SnapshotStorage) SendData(ctx context.Context, sensorID string, metadata map[string]string) error {
	return s.underlying.SendData(ctx, sensorID, metadata)
}

func (s *SnapshotStorage) GetStatus(ctx context.Context, sensorID string) (*SensorState, error) {
	return s.underlying.GetStatus(ctx, sensorID)
}

func (s *SnapshotStorage) Query(ctx context.Context, filter QueryFilter) ([]*SensorState, error) {
	return s.underlying.Query(ctx, filter)
}

func (s *SnapshotStorage) GetIdentity(ctx context.Context, sensorID string) (*SensorIdentity, error) {
	return s.underlying.GetIdentity(ctx, sensorID)
}

func (s *SnapshotStorage) FindIdentity(ctx context.Context, namespace, name string) (*SensorIdentity, error) {
	return s.underlying.FindIdentity(ctx, namespace, name)
}
