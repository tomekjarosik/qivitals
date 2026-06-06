package storage

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	v1 "github.com/tomekjarosik/qivitals/gen/api/qivitals/v1"
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
		return ErrSensorAlreadyExists
	}

	// Enforce unique (Namespace, Name) constraint
	for _, existing := range m.sensors {
		if existing.Info.Name == sensor.Name && existing.Info.Namespace == sensor.Namespace {
			return ErrSensorAlreadyExists
		}
	}

	// Deep copy labels
	labels := make(map[string]string)
	for k, v := range sensor.Labels {
		labels[k] = v
	}

	m.sensors[sensor.ID] = &SensorState{
		Info: &SensorInfo{
			ID:              sensor.ID,
			Name:            sensor.Name,
			Namespace:       sensor.Namespace,
			ResourceVersion: sensor.ResourceVersion,
			Description:     sensor.Description,
			GracefulPeriod:  sensor.GracefulPeriod,
			FailurePeriod:   sensor.FailurePeriod,
			Labels:          labels,
			RegisteredAt:    sensor.RegisteredAt,
			ConditionRules:  sensor.ConditionRules,
		},
		LastReportedAt: sensor.RegisteredAt,
		ReportedData:   make(map[string]string),
	}

	return nil
}

// Update modifies an existing sensor by its unique ID
func (m *MemorySensorStorage) Patch(ctx context.Context, sensorID string, expectedVersion string, updates *SensorInfo, columns []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.sensors[sensorID]
	if !exists {
		return ErrSensorNotFound
	}

	if expectedVersion != state.Info.ResourceVersion {
		return ErrVersionMismatch
	}

	for _, field := range columns {
		switch field {
		case "name":
			state.Info.Name = updates.Name
		case "namespace":
			state.Info.Namespace = updates.Namespace
		case "description":
			state.Info.Description = updates.Description
		case "graceful_period_seconds":
			state.Info.GracefulPeriod = updates.GracefulPeriod
		case "failure_period_seconds":
			state.Info.FailurePeriod = updates.FailurePeriod
		case "labels":
			labels := make(map[string]string)
			for k, v := range updates.Labels {
				labels[k] = v
			}
			state.Info.Labels = labels
		case "condition_rules":
			conditionRules := make([]*v1.ConditionRule, len(updates.ConditionRules))
			for i, rule := range updates.ConditionRules {
				conditionRules[i] = rule
			}
			state.Info.ConditionRules = conditionRules
		}
	}

	state.Info.LastSpecUpdatedAt = time.Now().Unix()
	state.Info.ResourceVersion = uuid.New().String()
	return nil
}

// SendData updates the last OK timestamp and last update timestamp for a sensor
func (m *MemorySensorStorage) SendData(ctx context.Context, sensorID string, metadata map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.sensors[sensorID]
	if !exists {
		return ErrSensorNotFound
	}

	now := time.Now().Unix()

	state.LastReportedAt = now
	for k, v := range metadata {
		state.ReportedData[k] = v
	}

	return nil
}

// GetStatus returns the current status and metadata for a sensor
func (m *MemorySensorStorage) GetStatus(ctx context.Context, sensorID string) (*SensorState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.sensors[sensorID]
	if !exists {
		return nil, ErrSensorNotFound
	}

	return state, nil
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

		if filter.Name != "" && state.Info.Name != filter.Name {
			continue
		}

		if filter.Namespace != "" && state.Info.Namespace != filter.Namespace {
			continue
		}

		// 1. Free-text Search (case-insensitive substring match)
		if filter.Search != "" {
			searchLower := strings.ToLower(filter.Search)
			nameMatch := strings.Contains(strings.ToLower(state.Info.Name), searchLower)
			descMatch := strings.Contains(strings.ToLower(state.Info.Description), searchLower)
			if !nameMatch && !descMatch {
				continue
			}
		}

		// 2. Exact Label Value match
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

		// 3. Label Key Existence match
		if len(filter.HasLabelKeys) > 0 {
			if state.Info.Labels == nil {
				continue
			}
			for _, requiredKey := range filter.HasLabelKeys {
				if _, exists := state.Info.Labels[requiredKey]; !exists {
					continue outer
				}
			}
		}

		results = append(results, state)
	}

	// 4. Sorting
	if filter.OrderBy != "" {
		sort.Slice(results, func(i, j int) bool {
			a, b := results[i], results[j]
			var less bool

			switch filter.OrderBy {
			case "name":
				less = a.Info.Name < b.Info.Name
			case "last_reported":
				less = a.LastReportedAt < b.LastReportedAt
			default:
				// Default to registered time if unknown field
				less = a.Info.RegisteredAt < b.Info.RegisteredAt
			}

			if filter.OrderDesc {
				return !less // Reverse the sort order
			}
			return less
		})
	}

	// 5. Pagination (Limit/Offset)
	// Offset logic can be added later if you build a token-based pagination system
	if filter.Limit > 0 && len(results) > filter.Limit {
		results = results[:filter.Limit]
	}

	return results, nil
}

func (m *MemorySensorStorage) Delete(ctx context.Context, sensorID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sensors[sensorID]; !exists {
		return ErrSensorNotFound
	}

	delete(m.sensors, sensorID)
	return nil
}

// GetIdentity retrieves only the identity metadata for a sensor by ID.
func (m *MemorySensorStorage) GetIdentity(ctx context.Context, sensorID string) (*SensorIdentity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.sensors[sensorID]
	if !exists {
		return nil, ErrSensorNotFound
	}

	return &SensorIdentity{
		ID:        state.Info.ID,
		Name:      state.Info.Name,
		Namespace: state.Info.Namespace,
	}, nil
}

// FindIdentity retrieves sensor identity by unique Name and Namespace combination.
func (m *MemorySensorStorage) FindIdentity(ctx context.Context, namespace, name string) (*SensorIdentity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, state := range m.sensors {
		if state.Info.Namespace == namespace && state.Info.Name == name {
			return &SensorIdentity{
				ID:        state.Info.ID,
				Name:      state.Info.Name,
				Namespace: state.Info.Namespace,
			}, nil
		}
	}

	return nil, ErrSensorNotFound
}
