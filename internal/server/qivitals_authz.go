package server

import (
	"context"
	"slices"

	v1 "github.com/tomekjarosik/qivitals/gen/api/qivitals/v1"
	"github.com/tomekjarosik/qivitals/internal/auth"
	"github.com/tomekjarosik/qivitals/internal/canonicallog"
	"github.com/tomekjarosik/qivitals/internal/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SensorIdentityResolver resolves a sensor's identity from either an ID or a Name+Namespace pair.
type SensorIdentityResolver interface {
	ResolveIdentity(ctx context.Context, id, name, namespace string) (*storage.SensorIdentity, error)
}

// ServiceAuthMiddleware wraps a StatusServiceServer and enforces namespace-level
// authorization before delegating to the inner service - this is between server layers
type ServiceAuthMiddleware struct {
	v1.UnimplementedQiVitalsServiceServer
	inner                  v1.QiVitalsServiceServer
	sensorIdentityResolver SensorIdentityResolver
}

func NewAuthorizedService(inner v1.QiVitalsServiceServer, sensorIdentityResolver SensorIdentityResolver) *ServiceAuthMiddleware {
	return &ServiceAuthMiddleware{inner: inner, sensorIdentityResolver: sensorIdentityResolver}
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
	sensorIdentity, err := m.sensorIdentityResolver.ResolveIdentity(ctx, req.Id, req.Name, req.Namespace)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "sensor %s not found", req.Id)
	}
	if !slices.Contains(entity.Namespaces(), sensorIdentity.Namespace) {
		return &v1.ReportSensorResponse{}, status.Errorf(codes.PermissionDenied, "can't report data in namespace '%s'", sensorIdentity.Namespace)
	}
	return m.inner.ReportSensor(ctx, req)
}

// DeleteSensor look up target's namespace via NamespaceResolver, then check user access ---
func (m *ServiceAuthMiddleware) DeleteSensor(ctx context.Context, req *v1.DeleteSensorRequest) (*v1.DeleteSensorResponse, error) {
	user := auth.EntityFromContext(ctx)
	if user == nil {
		return nil, status.Error(codes.Unauthenticated, "deletion requires authentication")
	}
	canonicallog.AddField(ctx, "entity.subject", user.SubjectID())

	sensorIdentity, err := m.sensorIdentityResolver.ResolveIdentity(ctx, req.Id, "", "")
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "sensor %s not found", req.Id)
	}

	if !user.HasAccessToNamespace(sensorIdentity.Namespace) {
		return nil, status.Errorf(codes.PermissionDenied,
			"user %s cannot delete sensor in namespace %s",
			user.SubjectID(), sensorIdentity.Namespace)
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

	sensorIdentity, err := m.sensorIdentityResolver.ResolveIdentity(ctx, req.Id, "", "")
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "sensor %s not found", req.Id)
	}

	if !user.HasAccessToNamespace(sensorIdentity.Namespace) {
		return nil, status.Errorf(codes.PermissionDenied,
			"user %s cannot patch sensor in namespace %s",
			user.SubjectID(), sensorIdentity.Namespace)
	}

	return m.inner.PatchSensor(ctx, req)
}

// QuerySensors enforce the namespace filter at query time ---
func (m *ServiceAuthMiddleware) QuerySensors(ctx context.Context, req *v1.QuerySensorsRequest) (*v1.QuerySensorsResponse, error) {
	user := auth.EntityFromContext(ctx)
	if user != nil {
		canonicallog.AddField(ctx, "entity.subject", user.SubjectID())
	}
	// Unauthenticated users can only query the public namespace
	if user == nil {
		canonicallog.AddField(ctx, "namespace", "public")
		if req.Namespace != "" && req.Namespace != "public" {
			return nil, status.Error(codes.PermissionDenied,
				"unauthenticated users can only query the public namespace")
		}
		// Force namespace to "public" even if the client didn't specify it
		req.Namespace = "public"
		req.Id = ""
	}

	return m.inner.QuerySensors(ctx, req)
}
