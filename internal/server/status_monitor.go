package server

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	v1 "github.com/tomekjarosik/one-status/gen/api/statussvc/v1"
	"github.com/tomekjarosik/one-status/internal/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	if req.Sensor == nil || req.Sensor.Metadata == nil || req.Sensor.Spec == nil {
		return nil, errors.New("sensor metadata and spec are required")
	}

	sensorInfo := &storage.SensorInfo{
		ID:              req.Sensor.Metadata.Id,
		Name:            req.Sensor.Metadata.Name,
		Namespace:       req.Sensor.Metadata.Namespace,
		ResourceVersion: uuid.New().String(),
		Description:     req.Sensor.Metadata.Description,
		GracefulPeriod:  req.Sensor.Spec.GracefulPeriodSeconds,
		FailurePeriod:   req.Sensor.Spec.FailurePeriodSeconds,
		Labels:          req.Sensor.Metadata.Labels, // Maps directly now!
	}

	if sensorInfo.ID == "" {
		sensorInfo.ID = uuid.New().String()
	}

	if err := s.storage.Register(ctx, sensorInfo); err != nil {
		return nil, err
	}

	// Fetch the newly registered state to return it fully populated
	state, err := s.storage.GetStatus(ctx, sensorInfo.ID)
	if err != nil {
		return nil, err
	}

	return &v1.RegisterSensorResponse{
		Sensor: buildProtoSensor(state),
	}, nil
}

func (s *StatusMonitorService) ReportSensor(ctx context.Context, req *v1.ReportSensorRequest) (*v1.ReportSensorResponse, error) {
	// First resolve ID if natural key was used
	targetID := req.Id
	if targetID == "" && req.Namespace != "" && req.Name != "" {
		res, err := s.storage.Query(ctx, storage.QueryFilter{Namespace: req.Namespace, Name: req.Name, Limit: 1})
		if err != nil || len(res) == 0 {
			return nil, status.Errorf(codes.NotFound, "sensor not found")
		}
		targetID = res[0].Info.ID
	}

	if err := s.storage.SendData(ctx, targetID, req.Data); err != nil {
		return nil, err
	}

	// Fetch updated state to return to client
	state, err := s.storage.GetStatus(ctx, targetID)
	if err != nil {
		return nil, err
	}
	return &v1.ReportSensorResponse{
		Sensor: buildProtoSensor(state),
	}, nil
}

func (s *StatusMonitorService) DeleteSensor(ctx context.Context, req *v1.DeleteSensorRequest) (*v1.DeleteSensorResponse, error) {
	if err := s.storage.Delete(ctx, req.Id); err != nil {
		if errors.Is(err, storage.ErrSensorNotFound) {
			return nil, err // Returning gRPC error is standard
		}
		return nil, err
	}

	return &v1.DeleteSensorResponse{}, nil
}

func (s *StatusMonitorService) QuerySensors(ctx context.Context, req *v1.QuerySensorsRequest) (*v1.QuerySensorsResponse, error) {
	filter := storage.QueryFilter{
		ID:           req.Id,
		Namespace:    req.Namespace,
		Name:         req.Name,
		Search:       req.Search,
		Labels:       req.Labels, // Maps directly!
		HasLabelKeys: req.HasLabelKeys,
		Statuses:     req.Statuses,
		OrderBy:      req.OrderBy,
		OrderDesc:    req.OrderDesc,
		Limit:        int(req.PageSize),
	}

	states, err := s.storage.Query(ctx, filter)
	if err != nil {
		return nil, err
	}

	sensors := make([]*v1.Sensor, 0, len(states))
	for _, state := range states {
		protoSensor := buildProtoSensor(state)

		// Filter computed status if requested
		if len(req.Statuses) > 0 {
			statusMatch := false
			for _, allowedStatus := range req.Statuses {
				if protoSensor.Status.State == allowedStatus {
					statusMatch = true
					break
				}
			}
			if !statusMatch {
				continue
			}
		}

		sensors = append(sensors, protoSensor)
	}

	return &v1.QuerySensorsResponse{
		Sensors: sensors,
	}, nil
}

// --- Helpers ---

func buildProtoSensor(state *storage.SensorState) *v1.Sensor {
	computedState := calculateSensorStatus(state)

	labels := state.Info.Labels
	if labels == nil {
		labels = make(map[string]string)
	}

	return &v1.Sensor{
		Metadata: &v1.ObjectMeta{
			Id:              state.Info.ID,
			Namespace:       state.Info.Namespace,
			Name:            state.Info.Name,
			ResourceVersion: state.Info.ResourceVersion,
			Description:     state.Info.Description,
			Labels:          labels,
		},
		Spec: &v1.SensorSpec{
			GracefulPeriodSeconds: state.Info.GracefulPeriod,
			FailurePeriodSeconds:  state.Info.FailurePeriod,
		},
		Status: &v1.SensorStatus{
			State:                computedState,
			LastUpdatedTimestamp: state.LastUpdated,
			ReportedData:         state.Metadata,
		},
	}
}

func calculateSensorStatus(state *storage.SensorState) string {
	now := time.Now().Unix()
	age := now - state.LastUpdated

	if age < state.Info.GracefulPeriod {
		return "OK"
	}

	if age < state.Info.FailurePeriod {
		return "DEGRADED"
	}

	return "DEAD"
}
