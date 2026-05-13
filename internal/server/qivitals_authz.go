package server

import (
	"context"

	v1 "github.com/tomekjarosik/qivitals/gen/api/qivitals/v1"
	"github.com/tomekjarosik/qivitals/internal/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NamespaceResolver resolves the namespace for a sensor by ID.
type NamespaceResolver interface {
	GetSensorNamespace(ctx context.Context, id string) (string, error)
}

// ServiceAuthMiddleware wraps a StatusServiceServer and enforces namespace-level
// authorization before delegating to the inner service.
type ServiceAuthMiddleware struct {
	inner v1.QiVitalsServiceClient
	store NamespaceResolver
}

func NewAuthorizedService(inner v1.QiVitalsServiceClient, store NamespaceResolver) *ServiceAuthMiddleware {
	return &ServiceAuthMiddleware{inner: inner, store: store}
}

// mustEmbedUnimplementedStatusServiceServer is a sentinel to satisfy the proto
// generated interface contract. It does nothing.
func (m *ServiceAuthMiddleware) mustEmbedUnimplementedStatusServiceServer() {
	// Sentinel — no-op
}

// RegisterSensor user must have access to the target namespace
func (m *ServiceAuthMiddleware) RegisterSensor(ctx context.Context, req *v1.RegisterSensorRequest) (*v1.RegisterSensorResponse, error) {
	user := auth.AuthEntityFromContext(ctx)
	if user == nil {
		return nil, status.Error(codes.Unauthenticated, "registration requires authentication")
	}

	ns := ""
	if req.Sensor != nil && req.Sensor.Metadata != nil {
		ns = req.Sensor.Metadata.Namespace
	}
	if ns == "" {
		ns = "default"
	}

	if !user.HasAccessToNamespace(ns) {
		return nil, status.Errorf(codes.PermissionDenied,
			"user %s cannot register sensor in namespace %s", user.SubjectID(), ns)
	}

	return m.inner.RegisterSensor(ctx, req)
}

// --- ReportSensor: no namespace check needed in new auth model ---
// (Sensors are no longer a token type; report requests use their own auth)
func (m *ServiceAuthMiddleware) ReportSensor(ctx context.Context, req *v1.ReportSensorRequest) (*v1.ReportSensorResponse, error) {
	return m.inner.ReportSensor(ctx, req)
}

// --- DeleteSensor: look up target's namespace via NamespaceResolver, then check user access ---
func (m *ServiceAuthMiddleware) DeleteSensor(ctx context.Context, req *v1.DeleteSensorRequest) (*v1.DeleteSensorResponse, error) {
	user := auth.AuthEntityFromContext(ctx)
	if user == nil {
		return nil, status.Error(codes.Unauthenticated, "deletion requires authentication")
	}

	ns, err := m.store.GetSensorNamespace(ctx, req.Id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "sensor %s not found", req.Id)
	}

	if !user.HasAccessToNamespace(ns) {
		return nil, status.Errorf(codes.PermissionDenied,
			"user %s cannot delete sensor in namespace %s",
			user.SubjectID(), ns)
	}

	return m.inner.DeleteSensor(ctx, req)
}

// --- PatchSensor: same pattern as DeleteSensor ---
func (m *ServiceAuthMiddleware) PatchSensor(ctx context.Context, req *v1.PatchSensorRequest) (*v1.PatchSensorResponse, error) {
	user := auth.AuthEntityFromContext(ctx)
	if user == nil {
		return nil, status.Error(codes.Unauthenticated, "patching requires authentication")
	}

	ns, err := m.store.GetSensorNamespace(ctx, req.Id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "sensor %s not found", req.Id)
	}

	if !user.HasAccessToNamespace(ns) {
		return nil, status.Errorf(codes.PermissionDenied,
			"user %s cannot patch sensor in namespace %s",
			user.SubjectID(), ns)
	}

	return m.inner.PatchSensor(ctx, req)
}

// --- QuerySensors: enforce namespace filter at query time ---
func (m *ServiceAuthMiddleware) QuerySensors(ctx context.Context, req *v1.QuerySensorsRequest) (*v1.QuerySensorsResponse, error) {
	user := auth.AuthEntityFromContext(ctx)

	// If user specified a namespace, verify they have access.
	if req.Namespace != "" {
		if user != nil && !user.HasAccessToNamespace(req.Namespace) {
			return nil, status.Errorf(codes.PermissionDenied,
				"user %s not allowed to query namespace %s",
				user.SubjectID(), req.Namespace)
		}
	}

	// If user is restricted (not admin), force namespace filter to their allowed set.
	if user != nil && len(user.Namespaces()) > 0 {
		if req.Namespace == "" {
			// User didn't specify a namespace — restrict to their allowed ones.
			if len(user.Namespaces()) == 1 {
				req.Namespace = user.Namespaces()[0]
			} else {
				// Multi-tenant user: reject ambiguous queries without explicit namespace.
				return nil, status.Errorf(codes.PermissionDenied,
					"user %s must specify a namespace (allowed: %v)",
					user.SubjectID(), user.Namespaces())
			}
		}
	}

	return m.inner.QuerySensors(ctx, req)
}
