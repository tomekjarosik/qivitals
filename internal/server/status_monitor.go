package server

import (
	"context"
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
		ID:             req.Sensor.SensorId,
		Name:           req.Sensor.SensorName,
		Description:    req.Sensor.Description,
		GracefulPeriod: req.Sensor.GracefulPeriodSeconds,
		FailurePeriod:  req.Sensor.FailurePeriodSeconds,
		Labels:         convertLabels(req.Sensor.Labels),
	}

	if sensor.ID == "" {
		sensor.ID = uuid.New().String()
	}

	if err := s.storage.Register(sensor); err != nil {
		if _, ok := err.(*storage.DuplicateSensorError); ok {
			timestamp := time.Now().Unix()
			return &v1.RegisterSensorResponse{
				SensorId:  sensor.ID,
				Success:   false,
				Timestamp: timestamp,
			}, err
		}
		return nil, err
	}

	timestamp := time.Now().Unix()
	return &v1.RegisterSensorResponse{
		SensorId:  sensor.ID,
		Success:   true,
		Timestamp: timestamp,
	}, nil
}

func (s *StatusMonitorService) ReportSensor(ctx context.Context, req *v1.ReportSensorRequest) (*v1.ReportSensorResponse, error) {
	if err := s.storage.SendData(req.SensorId, true, req.Data); err != nil {
		if _, ok := err.(*storage.SensorNotFoundError); ok {
			timestamp := time.Now().Unix()
			return &v1.ReportSensorResponse{
				SensorId:  req.SensorId,
				Success:   false,
				Timestamp: timestamp,
			}, err
		}
		return nil, err
	}

	timestamp := time.Now().Unix()
	return &v1.ReportSensorResponse{
		SensorId:  req.SensorId,
		Success:   true,
		Timestamp: timestamp,
	}, nil
}

func (s *StatusMonitorService) QuerySensors(ctx context.Context, req *v1.QuerySensorsRequest) (*v1.QuerySensorsResponse, error) {
	sensorIDs, err := s.storage.QueryByPath(req.Path)
	if err != nil {
		return nil, err
	}

	statuses := make([]*v1.SensorStatus, 0)

	for _, sensorID := range sensorIDs {
		state, err := s.storage.GetStatus(sensorID)
		if err != nil {
			continue
		}

		sensorStatus := calculateSensorStatus(state)

		statuses = append(statuses, &v1.SensorStatus{
			SensorId:             state.Info.ID,
			Status:               sensorStatus,
			LastOkTimestamp:      state.LastOkTimestamp,
			LastUpdatedTimestamp: state.LastUpdated,
		})
	}

	return &v1.QuerySensorsResponse{
		Sensors: statuses,
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
