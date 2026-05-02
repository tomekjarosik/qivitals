package storage

import (
	"sync"
	"time"
)

// SensorStatusType represents the current state of a sensor
type SensorStatusType string

const (
	StatusActive   SensorStatusType = "ACTIVE"
	StatusDegraded SensorStatusType = "DEGRADED"
	StatusDead     SensorStatusType = "DEAD"
)

// SensorInfo contains all information about a registered sensor
type SensorInfo struct {
	ID               string
	Name             string
	Description      string
	GracefulPeriod   int64
	FailurePeriod    int64
	Labels           map[string]string
	RegisteredAt     int64
}

// SensorState tracks the current state of a sensor
type SensorState struct {
	Info            *SensorInfo
	LastOkTimestamp int64
	LastUpdated     int64
	Metadata        map[string]string
}

// SensorStorage defines the interface for sensor persistence
type SensorStorage interface {
	Register(sensor *SensorInfo) error
	SendData(sensorID string, ok bool, metadata map[string]string) error
	GetStatus(sensorID string) (*SensorState, error)
	QueryByPath(path string) ([]string, error)
	QueryByLabels(labels map[string]string) ([]string, error)
	QueryAll() ([]string, error)
}

// MemorySensorStorage implements SensorStorage using in-memory maps
type MemorySensorStorage struct {
	sensors        map[string]*SensorState
	labelsIndex    map[string]map[string]bool
	pathIndex      map[string]map[string]bool
	mu             sync.RWMutex
}

// NewMemorySensorStorage creates a new in-memory sensor storage
func NewMemorySensorStorage() *MemorySensorStorage {
	return &MemorySensorStorage{
		sensors:    make(map[string]*SensorState),
		labelsIndex: make(map[string]map[string]bool),
		pathIndex:   make(map[string]map[string]bool),
	}
}

// Register adds a new sensor to the storage
func (m *MemorySensorStorage) Register(sensor *SensorInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sensors[sensor.ID]; exists {
		return &DuplicateSensorError{SensorID: sensor.ID}
	}

	now := time.Now().Unix()

	sensorState := &SensorState{
		Info: &SensorInfo{
			ID:             sensor.ID,
			Name:           sensor.Name,
			Description:    sensor.Description,
			GracefulPeriod: sensor.GracefulPeriod,
			FailurePeriod:  sensor.FailurePeriod,
			Labels:         make(map[string]string),
			RegisteredAt:   now,
		},
		LastOkTimestamp: now,
		LastUpdated:     now,
		Metadata:        make(map[string]string),
	}

	// Store sensor
	m.sensors[sensor.ID] = sensorState

	// Index by labels
	if sensor.Labels != nil {
		if m.labelsIndex[sensor.ID] == nil {
			m.labelsIndex[sensor.ID] = make(map[string]bool)
		}
		for key, value := range sensor.Labels {
			m.labelsIndex[sensor.ID][key] = true
			if m.labelsIndex[key] == nil {
				m.labelsIndex[key] = make(map[string]bool)
			}
			m.labelsIndex[key][value]	= true
		}
	}

	// Index by path (using sensor name as base path)
	if m.pathIndex[sensor.ID] == nil {
		m.pathIndex[sensor.ID] = make(map[string]bool)
	}
	m.pathIndex[sensor.ID][sensor.ID] = true
	if sensor.Name != "" {
		m.pathIndex[sensor.ID][sensor.Name] = true
	}

	return nil
}

// SendData updates the last OK timestamp and last update timestamp for a sensor
func (m *MemorySensorStorage) SendData(sensorID string, ok bool, metadata map[string]string) error {
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
func (m *MemorySensorStorage) GetStatus(sensorID string) (*SensorState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.sensors[sensorID]
	if !exists {
		return nil, &SensorNotFoundError{SensorID: sensorID}
	}

	return state, nil
}

// QueryByPath returns sensor IDs matching the given path pattern
func (m *MemorySensorStorage) QueryByPath(path string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []string

	m.mu.RLock()
	defer m.mu.RUnlock()

	if path == "" {
		for id := range m.sensors {
			results = append(results, id)
		}
		return results, nil
	}

	for id, paths := range m.pathIndex {
		for foundPath := range paths {
			if matchesPath(foundPath, path) {
				results = append(results, id)
				break
			}
		}
	}

	return results, nil
}

// QueryByLabels returns sensor IDs that have all specified labels
func (m *MemorySensorStorage) QueryByLabels(labels map[string]string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	targetLabels := labels
	if targetLabels == nil {
		targetLabels = make(map[string]string)
	}

	var results []string

outer:
	for sensorID, sensorState := range m.sensors {
		if sensorState.Info.Labels == nil {
			continue
		}

		for key, value := range targetLabels {
			if sensorState.Info.Labels[key] != value {
				continue outer
			}
		}

		results = append(results, sensorID)
	}

	return results, nil
}

// QueryAll returns all registered sensor IDs
func (m *MemorySensorStorage) QueryAll() ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []string
	for id := range m.sensors {
		results = append(results, id)
	}
	return results, nil
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

// DuplicateSensorError is returned when trying to register a duplicate sensor
type DuplicateSensorError struct {
	SensorID string
}

func (e *DuplicateSensorError) Error() string {
	return "sensor with ID " + e.SensorID + " already exists"
}

// SensorNotFoundError is returned when querying a non-existent sensor
type SensorNotFoundError struct {
	SensorID string
}

func (e *SensorNotFoundError) Error() string {
	return "sensor with ID " + e.SensorID + " not found"
}