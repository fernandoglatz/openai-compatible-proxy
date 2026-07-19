package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/constants"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/exceptions"
	"fernandoglatz/openai-compatible-proxy/internal/core/entity"
	"strings"
	"time"

	"github.com/google/uuid"
)

const modelColumns = `id, created_at, updated_at, name, object, type, publisher, arch,
	compatibility_type, quantization, state, max_context_length, display_name,
	size_bytes, params_string, capabilities, loaded_instance_ids`

type ModelRepository struct {
	db *sql.DB
}

func NewModelRepository() *ModelRepository {
	return &ModelRepository{db: utils.Database.Client}
}

// NewModelRepositoryWithDB builds a repository over an explicit handle, so tests can run
// against a temporary database instead of the process-wide one.
func NewModelRepositoryWithDB(db *sql.DB) *ModelRepository {
	return &ModelRepository{db: db}
}

func (repository *ModelRepository) Get(ctx context.Context, id string) (entity.Model, *exceptions.WrappedError) {
	return repository.getBy(ctx, "id", id)
}

func (repository *ModelRepository) GetByName(ctx context.Context, name string) (entity.Model, *exceptions.WrappedError) {
	return repository.getBy(ctx, "name", name)
}

func (repository *ModelRepository) getBy(ctx context.Context, column string, value string) (entity.Model, *exceptions.WrappedError) {
	var model entity.Model

	query := "SELECT " + modelColumns + " FROM models WHERE " + column + " = ? LIMIT 1"
	row := repository.db.QueryRowContext(ctx, query, value)

	err := repository.scan(row, &model)
	if errors.Is(err, sql.ErrNoRows) {
		return model, &exceptions.WrappedError{
			BaseError: exceptions.RecordNotFound,
		}
	} else if err != nil {
		return model, &exceptions.WrappedError{
			Error: err,
		}
	}

	repository.correctTimezone(&model)
	return model, nil
}

func (repository *ModelRepository) GetAll(ctx context.Context) ([]entity.Model, *exceptions.WrappedError) {
	var models []entity.Model = []entity.Model{}

	rows, err := repository.db.QueryContext(ctx, "SELECT "+modelColumns+" FROM models")
	if err != nil {
		return models, &exceptions.WrappedError{
			Error: err,
		}
	}

	defer rows.Close()

	for rows.Next() {
		var model entity.Model

		err = repository.scan(rows, &model)
		if err != nil {
			return models, &exceptions.WrappedError{
				Error: err,
			}
		}

		repository.correctTimezone(&model)
		models = append(models, model)
	}

	err = rows.Err()
	if err != nil {
		return models, &exceptions.WrappedError{
			Error: err,
		}
	}

	return models, nil
}

func (repository *ModelRepository) Save(ctx context.Context, model *entity.Model) *exceptions.WrappedError {
	now := time.Now()
	model.UpdatedAt = now

	if len(model.ID) == constants.ZERO {
		uuidObj, _ := uuid.NewRandom()
		uuidStr := uuidObj.String()
		model.ID = strings.Replace(uuidStr, "-", "", -1)
	}

	if model.CreatedAt.IsZero() {
		model.CreatedAt = now
	}

	capabilities, err := encodeStrings(model.Capabilities)
	if err != nil {
		return &exceptions.WrappedError{Error: err}
	}

	loadedInstanceIDs, err := encodeStrings(model.LoadedInstanceIDs)
	if err != nil {
		return &exceptions.WrappedError{Error: err}
	}

	// SQLite has no time type; storing RFC3339Nano in UTC keeps ordering lexicographic
	// and avoids depending on driver-implicit time conversion.
	args := []any{
		model.ID,
		formatTime(model.CreatedAt),
		formatTime(model.UpdatedAt),
		model.Name,
		model.Object,
		model.Type,
		model.Publisher,
		model.Arch,
		model.CompatibilityType,
		model.Quantization,
		model.State,
		model.MaxContextLength,
		model.DisplayName,
		model.SizeBytes,
		model.ParamsString,
		capabilities,
		loadedInstanceIDs,
	}

	query := `INSERT INTO models (` + modelColumns + `)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			name = excluded.name,
			object = excluded.object,
			type = excluded.type,
			publisher = excluded.publisher,
			arch = excluded.arch,
			compatibility_type = excluded.compatibility_type,
			quantization = excluded.quantization,
			state = excluded.state,
			max_context_length = excluded.max_context_length,
			display_name = excluded.display_name,
			size_bytes = excluded.size_bytes,
			params_string = excluded.params_string,
			capabilities = excluded.capabilities,
			loaded_instance_ids = excluded.loaded_instance_ids`

	_, err = repository.db.ExecContext(ctx, query, args...)
	if err != nil {
		return &exceptions.WrappedError{
			Error: err,
		}
	}

	return nil
}

func (repository *ModelRepository) Remove(ctx context.Context, model entity.Model) *exceptions.WrappedError {
	_, err := repository.db.ExecContext(ctx, "DELETE FROM models WHERE id = ?", model.ID)
	if err != nil {
		return &exceptions.WrappedError{
			Error: err,
		}
	}

	return nil
}

// scanner is satisfied by both *sql.Row and *sql.Rows, so single- and multi-row reads
// share one column mapping.
type scanner interface {
	Scan(dest ...any) error
}

func (repository *ModelRepository) scan(src scanner, model *entity.Model) error {
	var createdAt, updatedAt, capabilities, loadedInstanceIDs string

	err := src.Scan(
		&model.ID,
		&createdAt,
		&updatedAt,
		&model.Name,
		&model.Object,
		&model.Type,
		&model.Publisher,
		&model.Arch,
		&model.CompatibilityType,
		&model.Quantization,
		&model.State,
		&model.MaxContextLength,
		&model.DisplayName,
		&model.SizeBytes,
		&model.ParamsString,
		&capabilities,
		&loadedInstanceIDs,
	)
	if err != nil {
		return err
	}

	model.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return err
	}

	model.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return err
	}

	model.Capabilities, err = decodeStrings(capabilities)
	if err != nil {
		return err
	}

	model.LoadedInstanceIDs, err = decodeStrings(loadedInstanceIDs)
	if err != nil {
		return err
	}

	return nil
}

func (repository *ModelRepository) correctTimezone(model *entity.Model) {
	location, _ := time.LoadLocation(utils.GetTimezone())
	model.CreatedAt = model.CreatedAt.In(location)
	model.UpdatedAt = model.UpdatedAt.In(location)
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) (time.Time, error) {
	if len(value) == constants.ZERO {
		return time.Time{}, nil
	}

	return time.Parse(time.RFC3339Nano, value)
}

func encodeStrings(values []string) (string, error) {
	if values == nil {
		values = []string{}
	}

	encoded, err := json.Marshal(values)
	if err != nil {
		return "", err
	}

	return string(encoded), nil
}

func decodeStrings(value string) ([]string, error) {
	var values []string = []string{}

	if len(value) == constants.ZERO {
		return values, nil
	}

	err := json.Unmarshal([]byte(value), &values)
	if err != nil {
		return nil, err
	}

	return values, nil
}
