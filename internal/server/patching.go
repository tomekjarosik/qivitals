package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
	v1 "github.com/tomekjarosik/qivitals/gen/api/qivitals/v1"
	"github.com/tomekjarosik/qivitals/internal/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ---------------------------------------------------------------------------
// 1. Path Whitelist (K8s-style safety)
// ---------------------------------------------------------------------------

// patchablePaths defines which JSON Pointer paths are allowed to be modified.
// `true` means the path supports map-key sub-pathing (e.g. /metadata/labels/env).
var patchablePaths = map[string]bool{
	"/metadata/name":                false,
	"/metadata/namespace":           false,
	"/metadata/description":         false,
	"/metadata/labels":              true,
	"/spec/graceful_period_seconds": false,
	"/spec/failure_period_seconds":  false,
}

func validatePath(path string) error {
	if _, ok := patchablePaths[path]; ok {
		return nil
	}
	for prefix, allowsSub := range patchablePaths {
		if allowsSub && strings.HasPrefix(path, prefix+"/") {
			return nil
		}
	}
	return fmt.Errorf("path %q is not patchable", path)
}

// ---------------------------------------------------------------------------
// 2. The gRPC Handler
// ---------------------------------------------------------------------------

func (s *QiVitalsService) PatchSensor(
	ctx context.Context,
	req *v1.PatchSensorRequest,
) (*v1.PatchSensorResponse, error) {
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id required")
	}
	if len(req.GetOperations()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no patch operations provided")
	}

	// 1. Load current state
	currentState, err := s.storage.GetStatus(ctx, req.GetId())
	if err != nil {
		if errors.Is(err, storage.ErrSensorNotFound) {
			return nil, status.Errorf(codes.NotFound, "sensor not found: %v", err)
		}
		return nil, err
	}

	// 2. Validate paths and track which storage columns will change
	touchedColumns := map[string]bool{}
	for _, op := range req.GetOperations() {
		if err := validatePath(op.GetPath()); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		if col := pathToColumn(op.GetPath()); col != "" {
			touchedColumns[col] = true
		}
	}

	// 3. Marshal the current sensor into its API-shaped JSON document
	originalDoc, err := json.Marshal(sensorInfoToAPIShape(currentState.Info))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to marshal sensor: %v", err)
	}

	// 4. Convert proto operations to a json-patch RFC 6902 document
	patchBytes, err := buildJSONPatch(req.GetOperations())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid patch: %v", err)
	}

	patch, err := jsonpatch.DecodePatch(patchBytes)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to decode patch: %v", err)
	}

	// 5. Apply the patch (with safety options)
	opts := jsonpatch.NewApplyOptions()
	opts.AllowMissingPathOnRemove = true // Be lenient like Kubernetes
	opts.EnsurePathExistsOnAdd = true    // Auto-create intermediate maps

	patchedDoc, err := patch.ApplyWithOptions(originalDoc, opts)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "patch application failed: %v", err)
	}

	// 6. Convert back to typed storage struct
	updated, err := unmarshalAPIShape(patchedDoc, currentState.Info)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to deserialize: %v", err)
	}

	// 7. Persist only the touched columns
	columns := mapKeys(touchedColumns)
	if err := s.storage.Patch(ctx, req.GetId(), req.GetVersion(), updated, columns); err != nil {
		return nil, err
	}

	// 8. Return fresh state
	finalState, err := s.storage.GetStatus(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &v1.PatchSensorResponse{Sensor: buildProtoSensor(finalState)}, nil
}

// ---------------------------------------------------------------------------
// 3. Patch construction
// ---------------------------------------------------------------------------

// buildJSONPatch converts a slice of Protobuf PatchOperation into the standard
// RFC 6902 JSON Patch wire format that the json-patch library expects.
func buildJSONPatch(ops []*v1.PatchOperation) ([]byte, error) {
	type rfc6902Op struct {
		Op    string          `json:"op"`
		Path  string          `json:"path"`
		Value json.RawMessage `json:"value,omitempty"`
		From  string          `json:"from,omitempty"`
	}

	out := make([]rfc6902Op, 0, len(ops))
	for _, op := range ops {
		entry := rfc6902Op{
			Op:   op.GetOp(),
			Path: op.GetPath(),
		}
		// The proto carries `value` as a JSON-encoded string.
		// Operations like "remove" don't have a value.
		if v := op.GetValue(); v != "" {
			// Validate that it's parseable JSON before forwarding
			if !json.Valid([]byte(v)) {
				return nil, fmt.Errorf("op %q at %q has invalid JSON value", op.GetOp(), op.GetPath())
			}
			entry.Value = json.RawMessage(v)
		}
		out = append(out, entry)
	}
	return json.Marshal(out)
}

// ---------------------------------------------------------------------------
// 4. Shape Translation (Flat Storage <-> Nested API)
// ---------------------------------------------------------------------------

// apiShape mirrors the Protobuf hierarchy for JSON marshaling.
type apiShape struct {
	Metadata struct {
		ID          string            `json:"id"`
		Namespace   string            `json:"namespace"`
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Labels      map[string]string `json:"labels"`
	} `json:"metadata"`
	Spec struct {
		GracefulPeriod int64 `json:"graceful_period_seconds"`
		FailurePeriod  int64 `json:"failure_period_seconds"`
	} `json:"spec"`
}

func sensorInfoToAPIShape(info *storage.SensorInfo) *apiShape {
	out := &apiShape{}
	out.Metadata.ID = info.ID
	out.Metadata.Namespace = info.Namespace
	out.Metadata.Name = info.Name
	out.Metadata.Description = info.Description
	out.Metadata.Labels = cloneStringMap(info.Labels)
	out.Spec.GracefulPeriod = info.GracefulPeriod
	out.Spec.FailurePeriod = info.FailurePeriod
	return out
}

func unmarshalAPIShape(data []byte, original *storage.SensorInfo) (*storage.SensorInfo, error) {
	var shape apiShape
	if err := json.Unmarshal(data, &shape); err != nil {
		return nil, err
	}
	return &storage.SensorInfo{
		ID:             original.ID,           // immutable
		RegisteredAt:   original.RegisteredAt, // immutable
		Name:           shape.Metadata.Name,
		Namespace:      shape.Metadata.Namespace,
		Description:    shape.Metadata.Description,
		Labels:         shape.Metadata.Labels,
		GracefulPeriod: shape.Spec.GracefulPeriod,
		FailurePeriod:  shape.Spec.FailurePeriod,
	}, nil
}

// pathToColumn maps a JSON Pointer to a storage column name.
func pathToColumn(path string) string {
	parts := strings.SplitN(strings.TrimPrefix(path, "/"), "/", 3)
	if len(parts) < 2 {
		return ""
	}
	rootPath := "/" + parts[0] + "/" + parts[1]

	switch rootPath {
	case "/metadata/name":
		return "name"
	case "/metadata/namespace":
		return "namespace"
	case "/metadata/description":
		return "description"
	case "/metadata/labels":
		return "labels"
	case "/spec/graceful_period_seconds":
		return "graceful_period_seconds"
	case "/spec/failure_period_seconds":
		return "failure_period_seconds"
	}
	return ""
}

// ---------------------------------------------------------------------------
// 5. Utilities
// ---------------------------------------------------------------------------

func cloneStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func mapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
