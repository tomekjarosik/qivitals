package server

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"
	v1 "github.com/tomekjarosik/qivitals/gen/api/qivitals/v1"
	"github.com/tomekjarosik/qivitals/internal/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type QiVitalsService struct {
	v1.UnimplementedQiVitalsServiceServer
	storage            storage.SensorStorage
	conditionEvaluator *ConditionEvaluator
}

func NewQiVitalsService(storage storage.SensorStorage) *QiVitalsService {
	conditionEvaluator, err := NewConditionEvaluator()
	if err != nil {
		log.Fatalf("Warning: Failed to initialize CEL evaluator: %v", err)
	}

	return &QiVitalsService{
		storage:            storage,
		conditionEvaluator: conditionEvaluator,
	}
}

func (s *QiVitalsService) RegisterSensor(ctx context.Context, req *v1.RegisterSensorRequest) (*v1.RegisterSensorResponse, error) {
	if req.Sensor == nil || req.Sensor.Metadata == nil || req.Sensor.Spec == nil {
		return nil, errors.New("sensor metadata and spec are required")
	}
	now := time.Now().Unix()
	sensorInfo := &storage.SensorInfo{
		ID:              req.Sensor.Metadata.Id,
		Name:            req.Sensor.Metadata.Name,
		Namespace:       req.Sensor.Metadata.Namespace,
		ResourceVersion: uuid.New().String(),
		Description:     req.Sensor.Metadata.Description,
		GracefulPeriod:  req.Sensor.Spec.GracefulPeriodSeconds,
		FailurePeriod:   req.Sensor.Spec.FailurePeriodSeconds,
		Labels:          req.Sensor.Metadata.Labels, // Maps directly now!
		RegisteredAt:    now,
		ConditionRules:  req.Sensor.Spec.Rules,
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
	conditions := s.evaluateConditions(ctx, state)

	return &v1.RegisterSensorResponse{
		Sensor: buildProtoSensor(state, conditions),
	}, nil
}

func (s *QiVitalsService) evaluateConditions(ctx context.Context, state *storage.SensorState) []*v1.Condition {
	return s.conditionEvaluator.EvaluateConditions(
		ctx,
		state.Info.ConditionRules,
		state.ReportedData,
		state.Info.Labels,
	)
}

func (s *QiVitalsService) ResolveIdentity(ctx context.Context, id, name, namespace string) (*storage.SensorIdentity, error) {
	var identity *storage.SensorIdentity
	var err error

	if id != "" {
		identity, err = s.storage.GetIdentity(ctx, id)
	} else if name != "" && namespace != "" {
		identity, err = s.storage.FindIdentity(ctx, namespace, name)
	} else {
		return nil, status.Error(codes.InvalidArgument, "either id or name+namespace must be provided")
	}

	if err != nil {
		// Normalize storage errors to gRPC errors
		if errors.Is(err, storage.ErrSensorNotFound) {
			return nil, status.Error(codes.NotFound, "sensor not found")
		}
		return nil, err
	}

	return identity, nil
}

func (s *QiVitalsService) ReportSensor(ctx context.Context, req *v1.ReportSensorRequest) (*v1.ReportSensorResponse, error) {
	sensorIdentity, err := s.ResolveIdentity(ctx, req.Id, req.Name, req.Namespace)
	if err != nil {
		return nil, err
	}

	if err := s.storage.SendData(ctx, sensorIdentity.ID, req.Data); err != nil {
		return nil, err
	}

	state, err := s.storage.GetStatus(ctx, sensorIdentity.ID)
	if err != nil {
		return nil, err
	}
	conditions := s.evaluateConditions(ctx, state)
	sensor := buildProtoSensor(state, conditions)
	return &v1.ReportSensorResponse{Sensor: sensor}, nil
}

func (s *QiVitalsService) DeleteSensor(ctx context.Context, req *v1.DeleteSensorRequest) (*v1.DeleteSensorResponse, error) {
	identity, err := s.ResolveIdentity(ctx, req.Id, "", "")
	if err != nil {
		return nil, err
	}

	if err := s.storage.Delete(ctx, identity.ID); err != nil {
		if errors.Is(err, storage.ErrSensorNotFound) {
			return nil, status.Error(codes.NotFound, "sensor not found")
		}
		return nil, err
	}

	return &v1.DeleteSensorResponse{}, nil
}

func sensorStatesToStrings(states []v1.SensorState) []string {
	if len(states) == 0 {
		return nil
	}

	result := make([]string, len(states))
	for i, s := range states {
		result[i] = v1.SensorState_name[int32(s)]
	}

	return result
}

func (s *QiVitalsService) QuerySensors(ctx context.Context, req *v1.QuerySensorsRequest) (*v1.QuerySensorsResponse, error) {
	filter := storage.QueryFilter{
		ID:           req.Id,
		Namespace:    req.Namespace,
		Name:         req.Name,
		Search:       req.Search,
		Labels:       req.Labels, // Maps directly!
		HasLabelKeys: req.HasLabelKeys,
		States:       sensorStatesToStrings(req.States),
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
		conditions := s.evaluateConditions(ctx, state)
		protoSensor := buildProtoSensor(state, conditions)

		// Filter computed status if requested
		if len(req.States) > 0 {
			statusMatch := false
			for _, allowedStatus := range req.States {
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

func buildProtoSensor(state *storage.SensorState, conditions []*v1.Condition) *v1.Sensor {
	baseState := calculateSensorStatus(state)
	finalState := applyConditionOverrides(baseState, state, conditions)

	return &v1.Sensor{
		Metadata: &v1.ObjectMeta{
			Id:              state.Info.ID,
			Namespace:       state.Info.Namespace,
			Name:            state.Info.Name,
			ResourceVersion: state.Info.ResourceVersion,
			Description:     state.Info.Description,
			Labels:          state.Info.Labels,
		},
		Spec: &v1.SensorSpec{
			GracefulPeriodSeconds: state.Info.GracefulPeriod,
			FailurePeriodSeconds:  state.Info.FailurePeriod,
			Rules:                 state.Info.ConditionRules,
		},
		Status: &v1.SensorStatus{
			State:                 finalState,
			LastReportedTimestamp: state.LastReportedAt,
			ReportedData:          state.ReportedData,
			Conditions:            conditions,
		},
	}
}

func (s *QiVitalsService) GetSensor(ctx context.Context, id string) (*v1.Sensor, error) {
	state, err := s.storage.GetStatus(ctx, id)
	if err != nil {
		return nil, err
	}
	conditions := s.evaluateConditions(ctx, state)
	return buildProtoSensor(state, conditions), nil
}

func calculateSensorStatus(state *storage.SensorState) v1.SensorState {
	now := time.Now().Unix()
	age := now - state.LastReportedAt

	if age < state.Info.GracefulPeriod {
		return v1.SensorState_OK
	}

	if age < state.Info.FailurePeriod {
		return v1.SensorState_DEGRADED
	}

	return v1.SensorState_FAILED
}

// applyConditionOverrides evaluates active conditions and merges them with the base state.
// The final state is determined by the highest severity value (higher enum = more severe).
func applyConditionOverrides(baseState v1.SensorState, state *storage.SensorState, conditions []*v1.Condition) v1.SensorState {
	finalState := baseState

	for _, cond := range conditions {
		// Evaluation/parse errors force FAILED (trumps OK/DEGRADED, but respects PAUSED)
		if cond.Status == "Error" || cond.Status == "Unknown" {
			if v1.SensorState_FAILED > finalState {
				finalState = v1.SensorState_FAILED
			}
			continue
		}

		if cond.Status != "True" {
			continue
		}

		for _, rule := range state.Info.ConditionRules {
			if rule.Name == cond.Type && rule.TargetState != "" {
				if stateVal, ok := v1.SensorState_value[rule.TargetState]; ok {
					conditionState := v1.SensorState(stateVal)
					// Higher enum values represent more severe states and take precedence
					if conditionState > finalState {
						finalState = conditionState
					}
				}
				break
			}
		}
	}

	return finalState
}
