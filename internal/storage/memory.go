package storage

import (
	"context"
	"sync"
	"time"
)

// MemorySensorStorage implements SensorStorage using in-memory maps
type MemorySensorStorage struct {
	sensors map[string]*SensorState
	mu      sync.RWMutex
}

// NewMemorySensorStorage creates a new in-memory sensor storage
func NewMemorySensorStorage() *MemorySensorStorage {
	return &MemorySensorStorage{
		sensors: make(map[string]*SensorState),
	}
}

// Register adds a new sensor to the storage
func (m *MemorySensorStorage) Register(ctx context.Context, sensor *SensorInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sensors[sensor.ID]; exists {
		return &DuplicateSensorError{SensorID: sensor.ID}
	}

	// Enforce unique (Namespace, Name) constraint
	for _, existing := range m.sensors {
		// Note: Assuming you add Namespace to SensorInfo. If not, just check Name.
		if existing.Info.Name == sensor.Name {
			return &DuplicateSensorError{SensorID: sensor.Name + " (name already in use)"}
		}
	}

	now := time.Now().Unix()

	// Deep copy labels
	labels := make(map[string]string)
	for k, v := range sensor.Labels {
		labels[k] = v
	}

	m.sensors[sensor.ID] = &SensorState{
		Info: &SensorInfo{
			ID:             sensor.ID,
			Name:           sensor.Name,
			Description:    sensor.Description,
			GracefulPeriod: sensor.GracefulPeriod,
			FailurePeriod:  sensor.FailurePeriod,
			Labels:         labels,
			RegisteredAt:   now,
		},
		LastOkTimestamp: now,
		LastUpdated:     now,
		Metadata:        make(map[string]string),
	}

	return nil
}

// Update modifies an existing sensor by its unique ID
func (m *MemorySensorStorage) Update(ctx context.Context, sensorID string, updates *SensorInfo, updateMask []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.sensors[sensorID]
	if !exists {
		return &SensorNotFoundError{SensorID: sensorID}
	}

	for _, field := range updateMask {
		switch field {
		case "name":
			state.Info.Name = updates.Name
		case "description":
			state.Info.Description = updates.Description
		case "graceful_period_seconds": // matching proto field name convention
			state.Info.GracefulPeriod = updates.GracefulPeriod
		case "failure_period_seconds":
			state.Info.FailurePeriod = updates.FailurePeriod
		case "labels":
			labels := make(map[string]string)
			for k, v := range updates.Labels {
				labels[k] = v
			}
			state.Info.Labels = labels
		}
	}

	state.LastUpdated = time.Now().Unix()
	return nil
}

// SendData updates the last OK timestamp and last update timestamp for a sensor
func (m *MemorySensorStorage) SendData(ctx context.Context, sensorID string, ok bool, metadata map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.sensors[sensorID]
	if !exists {
		return &SensorNotFoundError{SensorID: sensorID}
	}

	now := time.Now().Unix()

	if ok {
		state.LastOkTimestamp = now
	}

	state.LastUpdated = now
	for k, v := range metadata {
		state.Metadata[k] = v
	}

	return nil
}

// GetStatus returns the current status and metadata for a sensor
func (m *MemorySensorStorage) GetStatus(ctx context.Context, sensorID string) (*SensorState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.sensors[sensorID]
	if !exists {
		return nil, &SensorNotFoundError{SensorID: sensorID}
	}

	return state, nil
}

// GetByNaturalKey allows the service layer to translate human inputs into an ID
func (m *MemorySensorStorage) GetByNaturalKey(ctx context.Context, namespace string, name string) (*SensorState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, state := range m.sensors {
		// Assuming Namespace gets added to SensorInfo.
		if state.Info.Name == name {
			return state, nil
		}
	}

	return nil, &SensorNotFoundError{SensorID: name}
}

// Query returns all sensors matching the broader filter criteria
func (m *MemorySensorStorage) Query(ctx context.Context, filter QueryFilter) ([]*SensorState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*SensorState

outer:
	for _, state := range m.sensors {
		if filter.ID != "" && state.Info.ID != filter.ID {
			continue
		}

		if filter.Path != "" && !matchesPath(state.Info.Name, filter.Path) {
			continue
		}

		if len(filter.Labels) > 0 {
			if state.Info.Labels == nil {
				continue
			}
			for k, v := range filter.Labels {
				if state.Info.Labels[k] != v {
					continue outer
				}
			}
		}

		// Note: Filtering by calculated Status requires evaluating the age here
		// If you want to support filter.Status, you'll need to calculate it for the state
		// and compare it, just like calculateSensorStatus does in your server layer.

		results = append(results, state)
	}

	return results, nil
}

func (m *MemorySensorStorage) Delete(ctx context.Context, sensorID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sensors[sensorID]; !exists {
		return &SensorNotFoundError{SensorID: sensorID}
	}

	delete(m.sensors, sensorID)
	return nil
}

// matchesPath checks if a path matches the given pattern
func matchesPath(foundPath, path string) bool {
	if path == "" {
		return true
	}
	if foundPath == path {
		return true
	}
	if len(path) > 0 && path[len(path)-1] == '*' && string(path[len(path)-1]) == "*" {
		prefix := path[:len(path)-1]
		if len(foundPath) >= len(prefix) && foundPath[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}
