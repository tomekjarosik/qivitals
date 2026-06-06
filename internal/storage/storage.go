package storage

import (
	"context"
	"errors"

	v1 "github.com/tomekjarosik/qivitals/gen/api/qivitals/v1"
)

// SensorStatusType represents the current state of a sensor
type SensorStatusType string

// SensorIdentity holds the minimal metadata required for lookup and authorization.
type SensorIdentity struct {
	ID        string
	Name      string
	Namespace string
}

// SensorInfo contains all information about a registered sensor
type SensorInfo struct {
	ID                string
	Namespace         string
	Name              string
	ResourceVersion   string
	Description       string
	GracefulPeriod    int64
	FailurePeriod     int64
	Labels            map[string]string
	RegisteredAt      int64
	LastSpecUpdatedAt int64
	ConditionRules    []*v1.ConditionRule
}

// SensorState tracks the current state of a sensor
type SensorState struct {
	Info           *SensorInfo
	ReportedData   map[string]string
	LastReportedAt int64
}

type QueryFilter struct {
	ID        string
	Namespace string
	Name      string

	// Advanced UI Filtering
	Search       string            // Matches substring in Name or Description
	Labels       map[string]string // Exact matches (AND logic)
	HasLabelKeys []string          // Key existence (AND logic)
	States       []string          // IN clause for computed states

	OrderBy   string
	OrderDesc bool
	Limit     int
	Cursor    string
}

var ErrSensorAlreadyExists = errors.New("sensor already exists")
var ErrSensorNotFound = errors.New("sensor not found")
var ErrVersionMismatch = errors.New("version mismatch")

// SensorStorage defines the interface for sensor persistence
type SensorStorage interface {
	// Register adds a new sensor. The DB should enforce a unique constraint
	// on (Namespace, Name) to prevent duplicates.
	Register(ctx context.Context, sensor *SensorInfo) error

	// Delete removes a sensor from the storage
	Delete(ctx context.Context, sensorID string) error

	// Patch modifies an existing sensor by its unique ID
	Patch(ctx context.Context, sensorID string, expectedVersion string, updates *SensorInfo, columns []string) error

	// SendData processes a heartbeat/signal using the unique ID
	SendData(ctx context.Context, sensorID string, metadata map[string]string) error

	// GetStatus retrieves a single sensor's full state by its unique ID
	GetStatus(ctx context.Context, sensorID string) (*SensorState, error)

	// Query returns all sensors matching the broader filter criteria
	Query(ctx context.Context, filter QueryFilter) ([]*SensorState, error)

	// GetIdentity retrieves only the identity metadata for a sensor by ID.
	// Optimized for authorization checks; does not fetch full specs or reported data.
	GetIdentity(ctx context.Context, sensorID string) (*SensorIdentity, error)

	// FindIdentity retrieves sensor identity by unique Name and Namespace combination.
	// Optimized for authorization checks; does not fetch full specs or reported data.
	FindIdentity(ctx context.Context, namespace, name string) (*SensorIdentity, error)
}
