package storage

import (
	"context"
	"errors"
)

// SensorStatusType represents the current state of a sensor
type SensorStatusType string

const (
	StatusActive   SensorStatusType = "OK"
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
	Version        string
}

// SensorState tracks the current state of a sensor
type SensorState struct {
	Info        *SensorInfo
	LastUpdated int64
	Metadata    map[string]string
}

type QueryFilter struct {
	ID        string
	Namespace string
	Name      string

	// Advanced UI Filtering
	Search       string            // Matches substring in Name or Description
	Labels       map[string]string // Exact matches (AND logic)
	HasLabelKeys []string          // Key existence (AND logic)
	Statuses     []string          // IN clause for computed statuses

	OrderBy   string
	OrderDesc bool
	Limit     int
	Cursor    string
}

var ErrSensorAlreadyExists = errors.New("sensor already exists")
var ErrSensorNotFound = errors.New("sensor not found")

// SensorStorage defines the interface for sensor persistence
type SensorStorage interface {
	// Register adds a new sensor. The DB should enforce a unique constraint
	// on (Namespace, Name) to prevent duplicates.
	Register(ctx context.Context, sensor *SensorInfo) error

	// Delete removes a sensor from the storage
	Delete(ctx context.Context, sensorID string) error

	// Patch modifies an existing sensor by its unique ID
	Patch(ctx context.Context, sensorID string, updates *SensorInfo, columns []string) error

	// SendData processes a heartbeat/signal using the unique ID
	SendData(ctx context.Context, sensorID string, metadata map[string]string) error

	// GetStatus retrieves a single sensor's full state by its unique ID
	GetStatus(ctx context.Context, sensorID string) (*SensorState, error)

	// Query returns all sensors matching the broader filter criteria
	Query(ctx context.Context, filter QueryFilter) ([]*SensorState, error)
}
