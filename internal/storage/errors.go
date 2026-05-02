package storage

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
