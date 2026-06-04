package server

import (
	"context"
	"slices"

	v1 "github.com/tomekjarosik/qivitals/gen/api/qivitals/v1"
	"github.com/tomekjarosik/qivitals/internal/auth"
	"github.com/tomekjarosik/qivitals/internal/canonicallog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SensorResolver resolves the namespace for a sensor by ID.
type SensorResolver interface {
	GetSensor(ctx context.Context, id string) (*v1.Sensor, error)
}

// ServiceAuthMiddleware wraps a StatusServiceServer and enforces namespace-level
// authorization before delegating to the inner service - this is between server layers
type ServiceAuthMiddleware struct {
	v1.UnimplementedQiVitalsServiceServer
	inner v1.QiVitalsServiceServer
	store SensorResolver
}

func NewAuthorizedService(inner v1.QiVitalsServiceServer, store SensorResolver) *ServiceAuthMiddleware {
	return &ServiceAuthMiddleware{inner: inner, store: store}
}

// RegisterSensor user must have access to the target namespace
func (m *ServiceAuthMiddleware) RegisterSensor(ctx context.Context, req *v1.RegisterSensorRequest) (*v1.RegisterSensorResponse, error) {
	user := auth.EntityFromContext(ctx)
	if user == nil {
		return nil, status.Error(codes.Unauthenticated, "registration requires authentication")
	}
	canonicallog.AddField(ctx, "entity.subject", user.SubjectID())

	ns := ""
	if req.Sensor != nil && req.Sensor.Metadata != nil {
		ns = req.Sensor.Metadata.Namespace
	}
	if ns == "" {
		ns = "default"
	}

	canonicallog.AddField(ctx, "namespace", ns)

	if !user.HasAccessToNamespace(ns) {
		return nil, status.Errorf(codes.PermissionDenied,
			"user %s cannot register sensor in namespace %s", user.SubjectID(), ns)
	}

	return m.inner.RegisterSensor(ctx, req)
}

func (m *ServiceAuthMiddleware) ReportSensor(ctx context.Context, req *v1.ReportSensorRequest) (*v1.ReportSensorResponse, error) {
	entity := auth.EntityFromContext(ctx)
	if entity == nil {
		return &v1.ReportSensorResponse{}, status.Error(codes.Unauthenticated, "no user found")
	}
	sensor, err := m.store.GetSensor(ctx, req.Id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "sensor %s not found", req.Id)
	}
	if slices.Contains(entity.Namespaces(), sensor.Metadata.Namespace) {
		return m.inner.ReportSensor(ctx, req)
	}
	return &v1.ReportSensorResponse{}, status.Errorf(codes.PermissionDenied, "can't report data in namespace '%s'", sensor.Metadata.Namespace)
}

// DeleteSensor look up target's namespace via NamespaceResolver, then check user access ---
func (m *ServiceAuthMiddleware) DeleteSensor(ctx context.Context, req *v1.DeleteSensorRequest) (*v1.DeleteSensorResponse, error) {
	user := auth.EntityFromContext(ctx)
	if user == nil {
		return nil, status.Error(codes.Unauthenticated, "deletion requires authentication")
	}
	canonicallog.AddField(ctx, "entity.subject", user.SubjectID())

	sensor, err := m.store.GetSensor(ctx, req.Id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "sensor %s not found", req.Id)
	}

	if !user.HasAccessToNamespace(sensor.Metadata.Namespace) {
		return nil, status.Errorf(codes.PermissionDenied,
			"user %s cannot delete sensor in namespace %s",
			user.SubjectID(), sensor.Metadata.Namespace)
	}

	return m.inner.DeleteSensor(ctx, req)
}

// PatchSensor same pattern as DeleteSensor
func (m *ServiceAuthMiddleware) PatchSensor(ctx context.Context, req *v1.PatchSensorRequest) (*v1.PatchSensorResponse, error) {
	user := auth.EntityFromContext(ctx)
	if user == nil {
		return nil, status.Error(codes.Unauthenticated, "patching requires authentication")
	}
	canonicallog.AddField(ctx, "entity.subject", user.SubjectID())

	sensor, err := m.store.GetSensor(ctx, req.Id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "sensor %s not found", req.Id)
	}

	if !user.HasAccessToNamespace(sensor.Metadata.Namespace) {
		return nil, status.Errorf(codes.PermissionDenied,
			"user %s cannot patch sensor in namespace %s",
			user.SubjectID(), sensor.Metadata.Namespace)
	}

	return m.inner.PatchSensor(ctx, req)
}

// QuerySensors enforce the namespace filter at query time ---
func (m *ServiceAuthMiddleware) QuerySensors(ctx context.Context, req *v1.QuerySensorsRequest) (*v1.QuerySensorsResponse, error) {
	user := auth.EntityFromContext(ctx)
	if user != nil {
		canonicallog.AddField(ctx, "entity.subject", user.SubjectID())
	}
	// TODO: Decide if we need to restrict namespace access

	return m.inner.QuerySensors(ctx, req)
}
