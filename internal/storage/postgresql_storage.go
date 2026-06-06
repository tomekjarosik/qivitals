package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	postgresUniqueViolation = "23505"
)

// PostgresSensorStorage implements SensorStorage using PostgreSQL
type PostgresSensorStorage struct {
	pool *pgxpool.Pool
	// squirrel.StatementBuilder automatically formats queries for Postgres ($1, $2, etc.)
	sq squirrel.StatementBuilderType
}

func NewPostgresSensorStorage(pool *pgxpool.Pool) *PostgresSensorStorage {
	return &PostgresSensorStorage{
		pool: pool,
		sq:   squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

// InitSchema creates the necessary table and indexes if they don't exist.
// In a real prod app, you'd use a migration tool (like golang-migrate), but this is great for tests/init.
func (p *PostgresSensorStorage) InitSchema(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS sensors (
		id VARCHAR(255) PRIMARY KEY,
		namespace VARCHAR(255) NOT NULL,
		name VARCHAR(255) NOT NULL,
        resource_version VARCHAR(255) NOT NULL,
		description TEXT,
		graceful_period BIGINT NOT NULL,
		failure_period BIGINT NOT NULL,
		labels JSONB NOT NULL DEFAULT '{}'::jsonb,
		registered_at BIGINT NOT NULL,
		last_reported BIGINT NOT NULL,
	    last_spec_updated BIGINT NOT NULL DEFAULT 0,
		metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
	    condition_rules JSONB DEFAULT '[]'::jsonb,
		CONSTRAINT unique_namespace_name UNIQUE (namespace, name)
	);
	CREATE INDEX IF NOT EXISTS idx_sensors_labels ON sensors USING GIN (labels);
	`
	_, err := p.pool.Exec(ctx, query)
	return err
}

func (p *PostgresSensorStorage) Register(ctx context.Context, sensor *SensorInfo) error {
	labelsJSON, _ := json.Marshal(sensor.Labels)
	if string(labelsJSON) == "null" {
		labelsJSON = []byte("{}")
	}
	conditionRulesJSON, _ := json.Marshal(sensor.ConditionRules)
	if string(conditionRulesJSON) == "null" {
		conditionRulesJSON = []byte("[]")
	}

	query, args, err := p.sq.Insert("sensors").
		Columns("id", "namespace", "name", "resource_version", "description", "graceful_period", "failure_period", "labels", "registered_at", "last_reported", "last_spec_updated", "metadata", "condition_rules").
		Values(sensor.ID, sensor.Namespace, sensor.Name, uuid.New().String(), sensor.Description, sensor.GracefulPeriod, sensor.FailurePeriod, labelsJSON, sensor.RegisteredAt, sensor.RegisteredAt, 0, "{}", conditionRulesJSON).
		ToSql()
	if err != nil {
		return err
	}

	_, err = p.pool.Exec(ctx, query, args...)
	if err != nil {
		// pgx specific way to check for Postgres Error codes
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == postgresUniqueViolation {
			return ErrSensorAlreadyExists
		}
		return err
	}

	return nil
}

func (p *PostgresSensorStorage) checkExists(ctx context.Context, id string) (bool, error) {
	var exists bool
	err := p.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM sensors WHERE id=$1)", id).Scan(&exists)
	return exists, err
}

func (p *PostgresSensorStorage) Patch(ctx context.Context, sensorID string, expectedVersion string, updates *SensorInfo, columns []string) error {
	if len(columns) == 0 {
		return nil // Nothing to update!
	}
	builder := p.sq.Update("sensors").Where(squirrel.Eq{"id": sensorID, "resource_version": expectedVersion})

	for _, column := range columns {
		switch column {
		case "name":
			builder = builder.Set("name", updates.Name)
		case "namespace":
			builder = builder.Set("namespace", updates.Namespace)
		case "description":
			builder = builder.Set("description", updates.Description)
		case "graceful_period_seconds":
			builder = builder.Set("graceful_period", updates.GracefulPeriod)
		case "failure_period_seconds":
			builder = builder.Set("failure_period", updates.FailurePeriod)
		case "labels":
			labelsJSON, _ := json.Marshal(updates.Labels)
			builder = builder.Set("labels", labelsJSON)
		case "condition_rules":
			rulesJSON, _ := json.Marshal(updates.ConditionRules)
			if string(rulesJSON) == "null" {
				rulesJSON = []byte("[]")
			}
			builder = builder.Set("condition_rules", rulesJSON)
		}
	}

	builder = builder.Set("resource_version", squirrel.Expr("gen_random_uuid()::text"))

	// Track when spec/metadata changes happen
	builder = builder.Set("last_spec_updated", squirrel.Expr("GREATEST(last_spec_updated, extract(epoch from now())::bigint)"))

	query, args, err := builder.ToSql()
	if err != nil {
		return err
	}

	tag, err := p.pool.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		exists, err := p.checkExists(ctx, sensorID)
		if err != nil {
			return err
		}
		if !exists {
			return ErrSensorNotFound
		}
		return ErrVersionMismatch
	}

	return nil
}

func (p *PostgresSensorStorage) SendData(ctx context.Context, sensorID string, metadata map[string]string) error {
	metaJSON, _ := json.Marshal(metadata)
	if string(metaJSON) == "null" {
		metaJSON = []byte("{}")
	}

	// Postgres JSONB magic: `metadata || $3` merges the new JSON map into the existing one!
	query := `
		UPDATE sensors 
		SET last_reported = extract(epoch from now()),
		    metadata = metadata || $1::jsonb
		WHERE id = $2
	`
	tag, err := p.pool.Exec(ctx, query, metaJSON, sensorID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrSensorNotFound
	}
	return nil
}

func (p *PostgresSensorStorage) GetStatus(ctx context.Context, sensorID string) (*SensorState, error) {
	filter := QueryFilter{ID: sensorID, Limit: 1}
	results, err := p.Query(ctx, filter)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, ErrSensorNotFound
	}
	return results[0], nil
}

func (p *PostgresSensorStorage) Delete(ctx context.Context, sensorID string) error {
	query, args, err := p.sq.Delete("sensors").Where(squirrel.Eq{"id": sensorID}).ToSql()
	if err != nil {
		return err
	}

	tag, err := p.pool.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrSensorNotFound
	}
	return nil
}

func (p *PostgresSensorStorage) Query(ctx context.Context, filter QueryFilter) ([]*SensorState, error) {
	builder := p.sq.Select("id", "namespace", "name", "resource_version", "description", "graceful_period", "failure_period", "labels", "registered_at", "last_reported", "last_spec_updated", "metadata", "condition_rules").From("sensors")

	if filter.ID != "" {
		builder = builder.Where(squirrel.Eq{"id": filter.ID})
	}
	if filter.Namespace != "" {
		builder = builder.Where(squirrel.Eq{"namespace": filter.Namespace})
	}
	if filter.Name != "" {
		builder = builder.Where(squirrel.Eq{"name": filter.Name})
	}

	if filter.Search != "" {
		searchTerm := "%" + filter.Search + "%"
		builder = builder.Where(squirrel.Or{
			squirrel.Expr("name ILIKE ?", searchTerm),
			squirrel.Expr("description ILIKE ?", searchTerm),
		})
	}

	if len(filter.Labels) > 0 {
		labelsJSON, _ := json.Marshal(filter.Labels)
		builder = builder.Where("labels @> ?::jsonb", string(labelsJSON))
	}

	if len(filter.HasLabelKeys) > 0 {
		// Replace "labels ?& ?" with the underlying function "jsonb_exists_all"
		// This prevents Squirrel from treating the operator's '?' as a placeholder token!
		builder = builder.Where("jsonb_exists_all(labels, ?::text[])", filter.HasLabelKeys)
	}

	orderByStr, err := sanitizeOrderBy(filter.OrderBy, filter.OrderDesc)
	if err != nil {
		return nil, err
	}
	if orderByStr != "" {
		builder = builder.OrderBy(orderByStr)
	}

	if filter.Limit > 0 {
		builder = builder.Limit(uint64(filter.Limit))
	}

	query, args, err := builder.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*SensorState
	for rows.Next() {
		var state SensorState
		var info SensorInfo
		var labelsBytes, metadataBytes, conditionRulesBytes []byte
		var lastSpecUpdated int64

		err := rows.Scan(
			&info.ID, &info.Namespace, &info.Name, &info.ResourceVersion, &info.Description,
			&info.GracefulPeriod, &info.FailurePeriod, &labelsBytes, &info.RegisteredAt,
			&state.LastReportedAt, &lastSpecUpdated, &metadataBytes, &conditionRulesBytes,
		)
		if err != nil {
			return nil, err
		}

		if err = json.Unmarshal(labelsBytes, &info.Labels); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(metadataBytes, &state.ReportedData); err != nil {
			return nil, err
		}
		if len(conditionRulesBytes) > 0 && !bytes.Equal(conditionRulesBytes, []byte("[]")) {
			if err = json.Unmarshal(conditionRulesBytes, &info.ConditionRules); err != nil {
				return nil, fmt.Errorf("failed to unmarshal condition_rules: %w", err)
			}
		}
		state.Info = &info
		results = append(results, &state)
	}

	return results, rows.Err()
}

var allowedOrderColumns = map[string]string{
	"name":      "name",
	"id":        "id",
	"namespace": "namespace",
}

func sanitizeOrderBy(column string, descending bool) (string, error) {
	if column == "" {
		return "", nil
	}
	col, ok := allowedOrderColumns[column]
	if !ok {
		return "", fmt.Errorf("unknown order column: %s", column)
	}
	direction := "ASC"
	if descending {
		direction = "DESC"
	}
	return fmt.Sprintf("%s %s", col, direction), nil
}

func (p *PostgresSensorStorage) GetIdentity(ctx context.Context, sensorID string) (*SensorIdentity, error) {
	query, args, err := p.sq.Select("id", "namespace", "name").
		From("sensors").
		Where(squirrel.Eq{"id": sensorID}).
		ToSql()
	if err != nil {
		return nil, err
	}

	var identity SensorIdentity
	err = p.pool.QueryRow(ctx, query, args...).Scan(
		&identity.ID,
		&identity.Namespace,
		&identity.Name,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSensorNotFound
		}
		return nil, err
	}
	return &identity, nil
}

func (p *PostgresSensorStorage) FindIdentity(ctx context.Context, namespace, name string) (*SensorIdentity, error) {
	query, args, err := p.sq.Select("id", "namespace", "name").
		From("sensors").
		Where(squirrel.Eq{"namespace": namespace, "name": name}).
		ToSql()
	if err != nil {
		return nil, err
	}

	var identity SensorIdentity
	err = p.pool.QueryRow(ctx, query, args...).Scan(
		&identity.ID,
		&identity.Namespace,
		&identity.Name,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSensorNotFound
		}
		return nil, err
	}
	return &identity, nil
}
