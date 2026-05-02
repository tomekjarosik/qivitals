package storage

import (
	"context"
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
	ID             string
	Name           string
	Namespace      string
	Description    string
	GracefulPeriod int64
	FailurePeriod  int64
	Labels         map[string]string
	RegisteredAt   int64
}

// SensorState tracks the current state of a sensor
type SensorState struct {
	Info            *SensorInfo
	LastOkTimestamp int64
	LastUpdated     int64
	Metadata        map[string]string
}

type QueryFilter struct {
	Namespace string
	ID        string
	Path      string // If you are keeping path-based matching
	Labels    map[string]string
	Status    string
}

// SensorStorage defines the interface for sensor persistence
type SensorStorage interface {
	// Register adds a new sensor. The DB should enforce a unique constraint
	// on (Namespace, Name) to prevent duplicates.
	Register(ctx context.Context, sensor *SensorInfo) error

	// Update modifies an existing sensor by its unique ID
	Update(ctx context.Context, sensorID string, updates *SensorInfo, updateMask []string) error

	// SendData processes a heartbeat/signal using the unique ID
	SendData(ctx context.Context, sensorID string, ok bool, metadata map[string]string) error

	// GetStatus retrieves a single sensor's full state by its unique ID
	GetStatus(ctx context.Context, sensorID string) (*SensorState, error)

	// GetByNaturalKey allows the service layer to translate human inputs into an ID
	GetByNaturalKey(ctx context.Context, namespace string, name string) (*SensorState, error)

	// Delete removes a sensor from the storage
	Delete(ctx context.Context, sensorID string) error

	// Query returns all sensors matching the broader filter criteria
	Query(ctx context.Context, filter QueryFilter) ([]*SensorState, error)
}
