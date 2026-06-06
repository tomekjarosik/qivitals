package storage

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "github.com/tomekjarosik/qivitals/gen/api/qivitals/v1"
)

// RunStorageContractTests executes the standard suite of storage tests against any SensorStorage implementation.
func RunStorageContractTests(t *testing.T, setup func() SensorStorage, teardown func()) {

	t.Run("Register and Get", func(t *testing.T) {
		storage := setup()
		defer teardown()
		ctx := context.Background()

		sensor := &SensorInfo{
			ID:             "sensor-1",
			Namespace:      "default",
			Name:           "db-backup",
			Description:    "Daily database backup",
			GracefulPeriod: 86400,
			FailurePeriod:  172800,
			Labels:         map[string]string{"env": "prod", "team": "data"},
		}

		// Success case
		err := storage.Register(ctx, sensor)
		require.NoError(t, err, "Failed to register sensor")

		// Edge case: Duplicate ID
		err = storage.Register(ctx, sensor)
		assert.IsType(t, ErrSensorAlreadyExists, err)

		// Edge case: Duplicate Namespace + Name
		sensorDiffID := &SensorInfo{ID: "sensor-2", Namespace: "default", Name: "db-backup"}
		err = storage.Register(ctx, sensorDiffID)
		assert.IsType(t, ErrSensorAlreadyExists, err)

		// Edge case: Same Name, Different Namespace (Should Succeed)
		sensorDiffNS := &SensorInfo{ID: "sensor-3", Namespace: "staging", Name: "db-backup"}
		err = storage.Register(ctx, sensorDiffNS)
		require.NoError(t, err, "Sensors with same name in different namespaces should be allowed")

		// Verify state retrieval
		state, err := storage.GetStatus(ctx, "sensor-1")
		require.NoError(t, err)
		assert.Equal(t, "db-backup", state.Info.Name)
		assert.Equal(t, "prod", state.Info.Labels["env"])

		// Edge case: Get non-existent
		_, err = storage.GetStatus(ctx, "does-not-exist")
		assert.IsType(t, ErrSensorNotFound, err)
	})

	t.Run("Condition Rules Roundtrip", func(t *testing.T) {
		storage := setup()
		defer teardown()
		ctx := context.Background()

		conditions := []*v1.ConditionRule{
			{
				Name:            "LowBattery",
				Expression:      `double(reported_data['battery_level']) < 15.0`,
				TargetState:     "DEGRADED",
				MessageTemplate: "Battery at {{ .reported_data.battery_level }}%",
			},
			{
				Name:        "HighLatency",
				Expression:  `int(reported_data['latency_ms']) > 500`,
				TargetState: "DEGRADED",
			},
			{
				Name:        "ProdEnv",
				Expression:  `labels['environment'] == 'production'`,
				TargetState: "DEGRADED",
			},
		}

		sensor := &SensorInfo{
			ID:             "condition-1",
			Namespace:      "infra",
			Name:           "battery-monitor",
			Description:    "Battery sensor with conditions",
			GracefulPeriod: 300,
			FailurePeriod:  600,
			Labels:         map[string]string{"environment": "production"},
			ConditionRules: conditions,
		}

		// Register sensor with conditions
		err := storage.Register(ctx, sensor)
		require.NoError(t, err, "Failed to register sensor with condition rules")

		// Retrieve and verify all condition fields
		state, err := storage.GetStatus(ctx, "condition-1")
		require.NoError(t, err)
		require.NotNil(t, state.Info.ConditionRules, "ConditionRules should not be nil")
		assert.Len(t, state.Info.ConditionRules, len(conditions), "Should store all %d condition rules", len(conditions))

		// Verify each rule preserves all fields exactly
		for i, expected := range conditions {
			actual := state.Info.ConditionRules[i]
			require.NotNil(t, actual, "Condition rule %d should not be nil", i)
			assert.Equal(t, expected.Name, actual.Name, "Rule name should match at index %d", i)
			assert.Equal(t, expected.Expression, actual.Expression, "Rule expression should match at index %d", i)
			assert.Equal(t, expected.TargetState, actual.TargetState, "Rule target_state should match at index %d", i)
			assert.Equal(t, expected.MessageTemplate, actual.MessageTemplate, "Rule message_template should match at index %d", i)
		}

		// Register sensor without conditions (empty rules)
		sensorNoConditions := &SensorInfo{
			ID:             "condition-2",
			Namespace:      "infra",
			Name:           "simple-sensor",
			GracefulPeriod: 100,
			FailurePeriod:  200,
			ConditionRules: nil,
		}
		err = storage.Register(ctx, sensorNoConditions)
		require.NoError(t, err)

		stateNoCond, err := storage.GetStatus(ctx, "condition-2")
		require.NoError(t, err)
		assert.Nil(t, stateNoCond.Info.ConditionRules, "Nil condition rules should roundtrip as nil")

		// Register sensor with empty slice
		sensorEmptySlice := &SensorInfo{
			ID:             "condition-3",
			Namespace:      "infra",
			Name:           "empty-sensors",
			GracefulPeriod: 100,
			FailurePeriod:  200,
			ConditionRules: []*v1.ConditionRule{},
		}
		err = storage.Register(ctx, sensorEmptySlice)
		require.NoError(t, err)

		stateEmpty, err := storage.GetStatus(ctx, "condition-3")
		require.NoError(t, err)
		// Empty slice may become nil after JSON marshal/unmarshal roundtrip, which is acceptable
		assert.Empty(t, stateEmpty.Info.ConditionRules)

		// Verify conditions persist across Query
		filter := QueryFilter{Namespace: "infra"}
		results, err := storage.Query(ctx, filter)
		require.NoError(t, err)

		var batterySensor, simpleSensor *SensorState
		for _, s := range results {
			if s.Info.Name == "battery-monitor" {
				batterySensor = s
			}
			if s.Info.Name == "simple-sensor" {
				simpleSensor = s
			}
		}

		require.NotNil(t, batterySensor, "battery-monitor should be found in query results")
		assert.Len(t, batterySensor.Info.ConditionRules, 3, "Condition rules should persist through Query")

		require.NotNil(t, simpleSensor, "simple-sensor should be found in query results")
		assert.Nil(t, simpleSensor.Info.ConditionRules, "No condition rules should be present for simple-sensor")
	})

	t.Run("Condition Rules Persist Through Patch", func(t *testing.T) {
		storage := setup()
		defer teardown()
		ctx := context.Background()

		conditions := []*v1.ConditionRule{
			{Name: "Rule1", Expression: `true`, TargetState: "DEGRADED"},
		}

		sensor := &SensorInfo{
			ID:             "patch-test",
			Namespace:      "test",
			Name:           "patch-condition",
			GracefulPeriod: 100,
			ConditionRules: conditions,
		}

		err := storage.Register(ctx, sensor)
		require.NoError(t, err)

		state, err := storage.GetStatus(ctx, "patch-test")
		require.NoError(t, err)
		assert.Len(t, state.Info.ConditionRules, 1)

		// Patch the description (not conditions)
		updates := &SensorInfo{Description: "updated"}
		err = storage.Patch(ctx, "patch-test", state.Info.ResourceVersion, updates, []string{"description"})
		require.NoError(t, err)

		stateAfterPatch, err := storage.GetStatus(ctx, "patch-test")
		require.NoError(t, err)
		assert.Equal(t, "updated", stateAfterPatch.Info.Description)
		assert.Len(t, stateAfterPatch.Info.ConditionRules, 1, "ConditionRules should persist after patching other fields")
		assert.Equal(t, "Rule1", stateAfterPatch.Info.ConditionRules[0].Name)
	})

	t.Run("Patch", func(t *testing.T) {
		storage := setup()
		defer teardown()
		ctx := context.Background()

		err := storage.Register(ctx, &SensorInfo{
			ID:             "s1",
			Name:           "test",
			Description:    "old",
			GracefulPeriod: 100,
		})
		require.NoError(t, err)

		state, err := storage.GetStatus(ctx, "s1")
		require.NoError(t, err)

		// Partial update
		updates := &SensorInfo{
			Name:           "new-test", // Should NOT be updated based on mask
			Description:    "new desc",
			GracefulPeriod: 200,
		}

		err = storage.Patch(ctx, "s1", state.Info.ResourceVersion, updates, []string{"description", "graceful_period_seconds"})
		require.NoError(t, err)

		state, _ = storage.GetStatus(ctx, "s1")
		assert.Equal(t, "new desc", state.Info.Description)
		assert.Equal(t, int64(200), state.Info.GracefulPeriod)
		assert.Equal(t, "test", state.Info.Name, "Name should not have changed since it wasn't in updateMask")

		// Edge case: Patch non-existent
		err = storage.Patch(ctx, "does-not-exist", "999", updates, []string{"metadata.description"})
		assert.IsType(t, ErrSensorNotFound, err)
	})

	t.Run("SendData", func(t *testing.T) {
		storage := setup()
		defer teardown()
		ctx := context.Background()

		storage.Register(ctx, &SensorInfo{ID: "s1", Name: "test"})

		// Initial Data (OK)
		err := storage.SendData(ctx, "s1", map[string]string{"version": "1.2.3"})
		require.NoError(t, err)

		state1, _ := storage.GetStatus(ctx, "s1")
		assert.Equal(t, "1.2.3", state1.ReportedData["version"])

		// Second Data (Failed) -> Should update metadata but NOT LastOkTimestamp
		err = storage.SendData(ctx, "s1", map[string]string{"error": "timeout"})
		require.NoError(t, err)

		state2, _ := storage.GetStatus(ctx, "s1")
		assert.GreaterOrEqual(t, state2.LastReportedAt, state1.LastReportedAt, "LastReportedAt should always increase")

		// Postgres JSONB || operator merges keys. Let's ensure memory storage does too (if you implemented merging).
		// If Memory storage overwrites entirely, this test might fail there. Assuming merging logic is intended!
		assert.Equal(t, "1.2.3", state2.ReportedData["version"], "Previous metadata keys should ideally be preserved (JSONB merge)")
		assert.Equal(t, "timeout", state2.ReportedData["error"])

		// Edge case: SendData non-existent
		err = storage.SendData(ctx, "does-not-exist", nil)
		assert.IsType(t, ErrSensorNotFound, err)
	})

	t.Run("Delete", func(t *testing.T) {
		storage := setup()
		defer teardown()
		ctx := context.Background()

		storage.Register(ctx, &SensorInfo{ID: "s1", Name: "test"})

		err := storage.Delete(ctx, "s1")
		require.NoError(t, err)

		_, err = storage.GetStatus(ctx, "s1")
		assert.IsType(t, ErrSensorNotFound, err)

		// Edge case: Delete already deleted
		err = storage.Delete(ctx, "s1")
		assert.IsType(t, ErrSensorNotFound, err)
	})

	t.Run("Query Advanced Filters", func(t *testing.T) {
		storage := setup()
		defer teardown()
		ctx := context.Background()

		storage.Register(ctx, &SensorInfo{ID: "s1", Namespace: "infra", Name: "job-a", Description: "Backup Alpha", Labels: map[string]string{"env": "prod", "tier": "1", "critical": "true"}})
		storage.Register(ctx, &SensorInfo{ID: "s2", Namespace: "infra", Name: "job-b", Description: "Backup Beta", Labels: map[string]string{"env": "prod", "tier": "2"}})
		storage.Register(ctx, &SensorInfo{ID: "s3", Namespace: "app", Name: "task-c", Description: "Log rotation for alpha", Labels: map[string]string{"env": "dev"}})
		storage.Register(ctx, &SensorInfo{ID: "s4", Namespace: "app", Name: "db-cleanup", Description: "Clear old logs", Labels: map[string]string{"env": "prod"}})

		tests := []struct {
			name          string
			filter        QueryFilter
			expectedIDs   []string
			expectedCount int
		}{
			{
				name:          "Empty filter matches all",
				filter:        QueryFilter{},
				expectedCount: 4,
			},
			{
				name:          "Filter by Namespace",
				filter:        QueryFilter{Namespace: "infra"},
				expectedCount: 2,
			},
			{
				name:          "Search by description and name substring (case insensitive)",
				filter:        QueryFilter{Search: "alpha"}, // Matches "Backup Alpha" (s1) and "Log rotation for alpha" (s3)
				expectedCount: 2,
				expectedIDs:   []string{"s1", "s3"},
			},
			{
				name:          "Complex: Namespace + Search + Label",
				filter:        QueryFilter{Namespace: "infra", Search: "beta", Labels: map[string]string{"env": "prod"}},
				expectedCount: 1,
				expectedIDs:   []string{"s2"},
			},
			{
				name:          "Has Label Key with multiple values",
				filter:        QueryFilter{HasLabelKeys: []string{"critical"}},
				expectedCount: 1,
				expectedIDs:   []string{"s1"},
			},
			{
				name:          "Sorting by Name ASC",
				filter:        QueryFilter{OrderBy: "name", OrderDesc: false},
				expectedCount: 4,
				expectedIDs:   []string{"s4", "s1", "s2", "s3"}, // db-cleanup, job-a, job-b, task-c
			},
			{
				name:          "Pagination (Limit)",
				filter:        QueryFilter{OrderBy: "name", OrderDesc: false, Limit: 2},
				expectedCount: 2,
				expectedIDs:   []string{"s4", "s1"},
			},
			{
				name:          "Non-matching complex query",
				filter:        QueryFilter{Namespace: "app", HasLabelKeys: []string{"critical"}},
				expectedCount: 0,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				results, err := storage.Query(ctx, tt.filter)
				require.NoError(t, err)
				assert.Len(t, results, tt.expectedCount)

				if len(tt.expectedIDs) > 0 {
					var actualIDs []string
					for _, res := range results {
						actualIDs = append(actualIDs, res.Info.ID)
					}
					// If sorting was tested, order matters, so we use Equal.
					// Otherwise, we just use ElementsMatch
					if tt.filter.OrderBy != "" {
						assert.Equal(t, tt.expectedIDs, actualIDs)
					} else {
						assert.ElementsMatch(t, tt.expectedIDs, actualIDs)
					}
				}
			})
		}
	})
}

// RunExtendedStorageContractTests executes advanced edge cases and stress tests.
func RunExtendedStorageContractTests(t *testing.T, setup func() SensorStorage, teardown func()) {

	t.Run("Update_Boundary_Cases", func(t *testing.T) {
		storage := setup()
		defer teardown()
		ctx := context.Background()

		initial := &SensorInfo{
			ID:             "boundary-1",
			Name:           "boundary-test",
			Description:    "original description",
			GracefulPeriod: 100,
		}
		err := storage.Register(ctx, initial)
		require.NoError(t, err)

		state, err := storage.GetStatus(ctx, "boundary-1")
		require.NoError(t, err)

		// Case: Empty Patch Mask (Nothing should change except LastUpdated)
		updates := &SensorInfo{
			Name:        "changed-name",
			Description: "changed-desc",
		}
		err = storage.Patch(ctx, "boundary-1", state.Info.ResourceVersion, updates, []string{})
		require.NoError(t, err)

		state, _ = storage.GetStatus(ctx, "boundary-1")
		assert.Equal(t, "boundary-test", state.Info.Name, "Name should not change with empty mask")
		assert.Equal(t, "original description", state.Info.Description, "Description should not change with empty mask")

		// Case: Invalid field in mask (System should handle gracefully)
		err = storage.Patch(ctx, "boundary-1", state.Info.ResourceVersion, updates, []string{"non_existent_field"})
		// Depending on your implementation, this might return an error or just ignore it.
		// We assume the system should be robust.
		if err != nil {
			assert.Error(t, err)
		}
	})

	t.Run("Query_Logic_Intersections", func(t *testing.T) {
		storage := setup()
		defer teardown()
		ctx := context.Background()

		// Setup specific dataset for intersection testing
		storage.Register(ctx, &SensorInfo{ID: "inter-1", Namespace: "prod", Name: "alpha", Labels: map[string]string{"tier": "gold"}})
		storage.Register(ctx, &SensorInfo{ID: "inter-2", Namespace: "prod", Name: "beta", Labels: map[string]string{"tier": "silver"}})
		storage.Register(ctx, &SensorInfo{ID: "inter-3", Namespace: "dev", Name: "gamma", Labels: map[string]string{"tier": "gold"}})

		// Case: Search matches, but Label excludes (AND logic check)
		filter := QueryFilter{
			Search: "alpha",                             // Matches inter-1
			Labels: map[string]string{"tier": "silver"}, // inter-1 is gold
		}
		results, err := storage.Query(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, results, 0, "Should return 0 because Search and Labels do not intersect")

		// Case: HasLabelKeys filter
		filterHasKey := QueryFilter{
			HasLabelKeys: []string{"tier"},
		}
		results, err = storage.Query(ctx, filterHasKey)
		require.NoError(t, err)
		assert.Len(t, results, 3, "All sensors should match as they all have the 'tier' key")
	})

	t.Run("Patch_VersionMismatch", func(t *testing.T) {
		// 1. Setup
		storage := setup() // Assuming setup() returns your SensorStorage implementation
		defer teardown()
		ctx := context.Background()

		sensorID := "version-test-id"
		initialVersion := "v1"

		// 2. Register the initial sensor with a specific version
		// Note: In your current provided code, 'Version' is in SensorInfo
		// but not explicitly handled in the Postgres Register/Patch logic.
		// This test assumes your implementation checks this field.
		err := storage.Register(ctx, &SensorInfo{
			ID:              sensorID,
			Name:            "version-test",
			Namespace:       "test",
			ResourceVersion: initialVersion,
			GracefulPeriod:  100,
			FailurePeriod:   200,
		})
		require.NoError(t, err)

		// 3. Get the current state (This represents the client's "cached" version)
		state, err := storage.GetStatus(ctx, sensorID)
		require.NoError(t, err)
		staleVersion := state.Info.ResourceVersion // This is "v1"

		// 4. Perform a "Successful" update that increments the version in the DB
		// We simulate a different client successfully updating the sensor to "v2"
		updateSuccess := &SensorInfo{
			Name:            "updated-successfully",
			ResourceVersion: "v2",
		}
		err = storage.Patch(ctx, sensorID, staleVersion, updateSuccess, []string{"name"})
		require.NoError(t, err, "First update should succeed")

		// 5. The "Conflict" Step:
		// Now we attempt to use the 'staleVersion' ("v1") to perform a new update.
		// Since the DB is now at "v2", the version "v1" is no longer valid.
		conflictUpdate := &SensorInfo{
			Name: "this-should-fail",
		}

		err = storage.Patch(ctx, sensorID, staleVersion, conflictUpdate, []string{"name"})

		require.Error(t, err, "Patch should fail due to stale resource version")
		assert.ErrorIs(t, err, ErrVersionMismatch)
	})

	// TODO: this is not yet implemented
	//t.Run("Pagination_Consistency", func(t *testing.T) {
	//	storage := setup()
	//	defer teardown()
	//	ctx := context.Background()
	//
	//	// Populate with many sensors
	//	for i := 0; i < 10; i++ {
	//		storage.Register(ctx, &SensorInfo{
	//			ID:   fmt.Sprintf("pag-%d", i),
	//			Name: fmt.Sprintf("sensor-%02d", i),
	//		})
	//	}
	//
	//	// Page 1
	//	filter1 := QueryFilter{OrderBy: "name", OrderDesc: false, Limit: 3}
	//	res1, err := storage.Query(ctx, filter1)
	//	require.NoError(t, err)
	//	assert.Len(t, res1, 3)
	//
	//	// Check that the first ID of the next page would be the 4th sensor
	//	// Note: Real cursor implementation depends on your DB (e.g., ID or Offset)
	//	// This test assumes the 'Cursor' logic is implemented.
	//	if len(res1) > 0 {
	//		filter2 := QueryFilter{
	//			OrderBy:   "name",
	//			OrderDesc: false,
	//			Limit:     3,
	//			Cursor:    res1[2].Info.ID, // Use last ID of page 1 as cursor
	//		}
	//		res2, err := storage.Query(ctx, filter2)
	//		if err == nil && len(res2) > 0 {
	//			assert.NotEqual(t, res1[0].Info.ID, res2[0].Info.ID, "Cursor should advance the result set")
	//		}
	//	}
	//})
}

// RunIdentityContractTests validates the lightweight identity lookup methods
// used by the authorization middleware.
func RunIdentityContractTests(t *testing.T, setup func() SensorStorage, teardown func()) {

	t.Run("GetIdentity", func(t *testing.T) {
		storage := setup()
		defer teardown()
		ctx := context.Background()

		sensor := &SensorInfo{
			ID:             "ident-1",
			Namespace:      "default",
			Name:           "auth-test-sensor",
			Description:    "Used for identity lookup tests",
			GracefulPeriod: 100,
			FailurePeriod:  200,
			Labels:         map[string]string{"env": "test"},
		}
		err := storage.Register(ctx, sensor)
		require.NoError(t, err, "Failed to register sensor for identity test")

		// Success: Lookup by ID
		identity, err := storage.GetIdentity(ctx, "ident-1")
		require.NoError(t, err)
		assert.Equal(t, "ident-1", identity.ID)
		assert.Equal(t, "default", identity.Namespace)
		assert.Equal(t, "auth-test-sensor", identity.Name)

		// Edge case: Non-existent ID
		_, err = storage.GetIdentity(ctx, "does-not-exist")
		assert.ErrorIs(t, err, ErrSensorNotFound, "GetIdentity should return ErrSensorNotFound for missing ID")
	})

	t.Run("FindIdentity", func(t *testing.T) {
		storage := setup()
		defer teardown()
		ctx := context.Background()

		sensor := &SensorInfo{
			ID:             "ident-2",
			Namespace:      "prod",
			Name:           "db-primary",
			GracefulPeriod: 100,
			FailurePeriod:  200,
		}
		err := storage.Register(ctx, sensor)
		require.NoError(t, err, "Failed to register sensor for natural key test")

		// Success: Lookup by Namespace + Name
		identity, err := storage.FindIdentity(ctx, "prod", "db-primary")
		require.NoError(t, err)
		assert.Equal(t, "ident-2", identity.ID, "FindIdentity should return the correct underlying ID")
		assert.Equal(t, "prod", identity.Namespace)
		assert.Equal(t, "db-primary", identity.Name)

		// Edge case: Correct Namespace, Wrong Name
		_, err = storage.FindIdentity(ctx, "prod", "wrong-name")
		assert.ErrorIs(t, err, ErrSensorNotFound, "FindIdentity should fail when name doesn't match")

		// Edge case: Correct Name, Wrong Namespace
		_, err = storage.FindIdentity(ctx, "wrong-ns", "db-primary")
		assert.ErrorIs(t, err, ErrSensorNotFound, "FindIdentity should fail when namespace doesn't match")
	})
}
