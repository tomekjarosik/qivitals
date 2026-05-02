package server

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/tomekjarosik/one-status/gen/api/statussvc/v1"
	"github.com/tomekjarosik/one-status/internal/storage"
)

type StatusMonitorService struct {
	v1.UnimplementedStatusServiceServer
	storage storage.SensorStorage
}

func NewStatusMonitorService(storage storage.SensorStorage) *StatusMonitorService {
	return &StatusMonitorService{
		storage: storage,
	}
}

func (s *StatusMonitorService) RegisterSensor(ctx context.Context, req *v1.RegisterSensorRequest) (*v1.RegisterSensorResponse, error) {
	sensor := &storage.SensorInfo{
		ID:             req.Spec.Id,
		Name:           req.Spec.Name,
		Description:    req.Spec.Description,
		GracefulPeriod: req.Spec.GracefulPeriodSeconds,
		FailurePeriod:  req.Spec.FailurePeriodSeconds,
		Labels:         convertLabels(req.Spec.Labels),
	}

	if sensor.ID == "" {
		sensor.ID = uuid.New().String()
	}

	// Pass context to the new storage interface
	if err := s.storage.Register(ctx, sensor); err != nil {
		var duplicateSensorError *storage.DuplicateSensorError
		if errors.As(err, &duplicateSensorError) {
			timestamp := time.Now().Unix()
			return &v1.RegisterSensorResponse{
				Id:        sensor.ID,
				Success:   false,
				Timestamp: timestamp,
			}, err
		}
		return nil, err
	}

	timestamp := time.Now().Unix()
	return &v1.RegisterSensorResponse{
		Id:        sensor.ID,
		Success:   true,
		Timestamp: timestamp,
	}, nil
}

func (s *StatusMonitorService) ReportSensor(ctx context.Context, req *v1.ReportSensorRequest) (*v1.ReportSensorResponse, error) {
	// Pass context to the new storage interface
	if err := s.storage.SendData(ctx, req.Id, true, req.Data); err != nil {
		var sensorNotFoundError *storage.SensorNotFoundError
		if errors.As(err, &sensorNotFoundError) {
			timestamp := time.Now().Unix()
			return &v1.ReportSensorResponse{
				Id:        req.Id,
				Success:   false,
				Timestamp: timestamp,
			}, err
		}
		return nil, err
	}

	timestamp := time.Now().Unix()
	return &v1.ReportSensorResponse{
		Id:        req.Id,
		Success:   true,
		Timestamp: timestamp,
	}, nil
}

func (s *StatusMonitorService) DeleteSensor(ctx context.Context, req *v1.DeleteSensorRequest) (*v1.DeleteSensorResponse, error) {
	if err := s.storage.Delete(ctx, req.Id); err != nil {
		var sensorNotFoundError *storage.SensorNotFoundError
		if errors.As(err, &sensorNotFoundError) {
			return &v1.DeleteSensorResponse{Success: false}, err
		}
		return nil, err
	}

	return &v1.DeleteSensorResponse{Success: true}, nil
}

func (s *StatusMonitorService) QuerySensors(ctx context.Context, req *v1.QuerySensorsRequest) (*v1.QuerySensorsResponse, error) {
	// Build the filter using the new storage.QueryFilter structure
	filter := storage.QueryFilter{
		Namespace: req.Namespace,
		ID:        req.Id,
		Labels:    convertLabels(req.Labels),
	}

	// Fetch all matching full states in ONE call
	states, err := s.storage.Query(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Map the storage models to the protobuf response models
	sensors := make([]*v1.Sensor, 0, len(states))
	for _, state := range states {
		computedState := calculateSensorStatus(state)

		// If the request specifically filtered by status, we can enforce it here
		if req.Status != "" && req.Status != computedState {
			continue
		}

		// Convert storage labels to proto labels
		var protoLabels []*v1.Label
		if state.Info.Labels != nil {
			for k, v := range state.Info.Labels {
				protoLabels = append(protoLabels, &v1.Label{Key: k, Value: v})
			}
		}

		// Build the Spec part
		spec := &v1.SensorSpec{
			Id:                    state.Info.ID,
			Name:                  state.Info.Name,
			Namespace:             state.Info.Namespace,
			Description:           state.Info.Description,
			GracefulPeriodSeconds: state.Info.GracefulPeriod,
			FailurePeriodSeconds:  state.Info.FailurePeriod,
			Labels:                protoLabels,
		}

		// Build the Status part, mapping the stored Metadata to ReportedData
		status := &v1.SensorStatus{
			State:                computedState,
			LastOkTimestamp:      state.LastOkTimestamp,
			LastUpdatedTimestamp: state.LastUpdated,
			ReportedData:         state.Metadata,
		}

		sensors = append(sensors, &v1.Sensor{
			Id:     state.Info.ID,
			Spec:   spec,
			Status: status,
		})
	}

	return &v1.QuerySensorsResponse{
		Sensors: sensors,
	}, nil
}

func calculateSensorStatus(state *storage.SensorState) string {
	now := time.Now().Unix()
	age := now - state.LastOkTimestamp

	if age < state.Info.GracefulPeriod {
		return "ACTIVE"
	}

	if age < state.Info.FailurePeriod {
		return "DEGRADED"
	}

	return "DEAD"
}

func convertLabels(labels []*v1.Label) map[string]string {
	if labels == nil {
		return make(map[string]string)
	}

	result := make(map[string]string)
	for _, label := range labels {
		result[label.Key] = label.Value
	}
	return result
}
