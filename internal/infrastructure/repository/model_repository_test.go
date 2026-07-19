package repository

import (
	"context"
	"database/sql"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/exceptions"
	"fernandoglatz/openai-compatible-proxy/internal/core/entity"
	"path/filepath"
	"testing"
	"time"
)

// newTestRepository opens a throwaway database and applies the real migrations, so the
// tests exercise the shipped schema rather than a hand-written copy of it.
func newTestRepository(t *testing.T) *ModelRepository {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
	migrations, err := filepath.Abs("../../../scripts/sqlite/migrations")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}

	db, err := utils.OpenDatabase(context.Background(), path, migrations)
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}

	t.Cleanup(func() { db.Close() })

	return NewModelRepositoryWithDB(db)
}

func TestSaveGeneratesIDAndTimestamps(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()

	model := entity.Model{Name: "llama-3"}
	wrapped := repository.Save(ctx, &model)
	if wrapped != nil {
		t.Fatalf("Save: %v", wrapped.Error)
	}

	if len(model.ID) != 32 {
		t.Errorf("ID = %q, want a 32-char hyphen-free UUID", model.ID)
	}

	if model.CreatedAt.IsZero() || model.UpdatedAt.IsZero() {
		t.Errorf("timestamps not set: created=%v updated=%v", model.CreatedAt, model.UpdatedAt)
	}
}

func TestSavePreservesSuppliedID(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()

	model := entity.Model{ID: "fixed-id", Name: "llama-3"}
	wrapped := repository.Save(ctx, &model)
	if wrapped != nil {
		t.Fatalf("Save: %v", wrapped.Error)
	}

	if model.ID != "fixed-id" {
		t.Errorf("ID = %q, want %q", model.ID, "fixed-id")
	}
}

// A second Save of an already-persisted model must update in place rather than insert a
// duplicate, so the row count is the assertion that matters.
func TestSaveUpdatesInPlace(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()

	model := entity.Model{Name: "llama-3", State: "not-loaded"}
	if wrapped := repository.Save(ctx, &model); wrapped != nil {
		t.Fatalf("first Save: %v", wrapped.Error)
	}

	createdAt := model.CreatedAt

	model.State = "loaded"
	if wrapped := repository.Save(ctx, &model); wrapped != nil {
		t.Fatalf("second Save: %v", wrapped.Error)
	}

	models, wrapped := repository.GetAll(ctx)
	if wrapped != nil {
		t.Fatalf("GetAll: %v", wrapped.Error)
	}

	if len(models) != 1 {
		t.Fatalf("row count = %d, want 1 (update, not insert)", len(models))
	}

	if models[0].State != "loaded" {
		t.Errorf("State = %q, want %q", models[0].State, "loaded")
	}

	if !models[0].CreatedAt.Equal(createdAt) {
		t.Errorf("CreatedAt changed on update: got %v, want %v", models[0].CreatedAt, createdAt)
	}
}

func TestRoundTripPreservesAllFields(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()

	model := entity.Model{
		Name:              "llama-3",
		Object:            "model",
		Type:              "llm",
		Publisher:         "meta",
		Arch:              "llama",
		CompatibilityType: "gguf",
		Quantization:      "Q4_K_M",
		State:             "loaded",
		MaxContextLength:  8192,
		DisplayName:       "Llama 3",
		SizeBytes:         4_920_000_000,
		ParamsString:      "8B",
		Capabilities:      []string{"tool_use", "vision"},
		LoadedInstanceIDs: []string{"instance-a", "instance-b"},
	}

	if wrapped := repository.Save(ctx, &model); wrapped != nil {
		t.Fatalf("Save: %v", wrapped.Error)
	}

	loaded, wrapped := repository.Get(ctx, model.ID)
	if wrapped != nil {
		t.Fatalf("Get: %v", wrapped.Error)
	}

	if loaded.Name != model.Name || loaded.Publisher != model.Publisher ||
		loaded.CompatibilityType != model.CompatibilityType || loaded.State != model.State {
		t.Errorf("string fields mismatch: got %+v", loaded)
	}

	if loaded.MaxContextLength != 8192 {
		t.Errorf("MaxContextLength = %d, want 8192", loaded.MaxContextLength)
	}

	if loaded.SizeBytes != 4_920_000_000 {
		t.Errorf("SizeBytes = %d, want 4920000000", loaded.SizeBytes)
	}

	if len(loaded.Capabilities) != 2 || loaded.Capabilities[0] != "tool_use" || loaded.Capabilities[1] != "vision" {
		t.Errorf("Capabilities = %v, want [tool_use vision]", loaded.Capabilities)
	}

	if len(loaded.LoadedInstanceIDs) != 2 || loaded.LoadedInstanceIDs[0] != "instance-a" {
		t.Errorf("LoadedInstanceIDs = %v, want [instance-a instance-b]", loaded.LoadedInstanceIDs)
	}

	// Timestamps survive the RFC3339Nano text round trip. Compare as instants: reads
	// come back in the configured timezone, not UTC.
	if !loaded.CreatedAt.Equal(model.CreatedAt.Truncate(time.Nanosecond)) {
		t.Errorf("CreatedAt = %v, want %v", loaded.CreatedAt, model.CreatedAt)
	}
}

// The LM Studio v0 path leaves the slice fields nil. They must read back as empty
// slices, not nil, because consumers range over them without a nil check.
func TestNilSlicesReadBackEmpty(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()

	model := entity.Model{Name: "legacy-model"}
	if wrapped := repository.Save(ctx, &model); wrapped != nil {
		t.Fatalf("Save: %v", wrapped.Error)
	}

	loaded, wrapped := repository.Get(ctx, model.ID)
	if wrapped != nil {
		t.Fatalf("Get: %v", wrapped.Error)
	}

	if loaded.Capabilities == nil || len(loaded.Capabilities) != 0 {
		t.Errorf("Capabilities = %v, want empty non-nil slice", loaded.Capabilities)
	}

	if loaded.LoadedInstanceIDs == nil || len(loaded.LoadedInstanceIDs) != 0 {
		t.Errorf("LoadedInstanceIDs = %v, want empty non-nil slice", loaded.LoadedInstanceIDs)
	}
}

func TestGetByName(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()

	model := entity.Model{Name: "qwen-2.5"}
	if wrapped := repository.Save(ctx, &model); wrapped != nil {
		t.Fatalf("Save: %v", wrapped.Error)
	}

	loaded, wrapped := repository.GetByName(ctx, "qwen-2.5")
	if wrapped != nil {
		t.Fatalf("GetByName: %v", wrapped.Error)
	}

	if loaded.ID != model.ID {
		t.Errorf("ID = %q, want %q", loaded.ID, model.ID)
	}
}

func TestMissingRowReturnsRecordNotFound(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()

	_, wrapped := repository.Get(ctx, "does-not-exist")
	if wrapped == nil || wrapped.BaseError != exceptions.RecordNotFound {
		t.Errorf("Get: got %v, want RecordNotFound", wrapped)
	}

	_, wrapped = repository.GetByName(ctx, "does-not-exist")
	if wrapped == nil || wrapped.BaseError != exceptions.RecordNotFound {
		t.Errorf("GetByName: got %v, want RecordNotFound", wrapped)
	}
}

func TestGetAllOnEmptyTableReturnsEmptySlice(t *testing.T) {
	repository := newTestRepository(t)

	models, wrapped := repository.GetAll(context.Background())
	if wrapped != nil {
		t.Fatalf("GetAll: %v", wrapped.Error)
	}

	if models == nil {
		t.Fatal("GetAll returned nil, want empty non-nil slice")
	}

	if len(models) != 0 {
		t.Errorf("len = %d, want 0", len(models))
	}
}

func TestRemove(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()

	model := entity.Model{Name: "temp"}
	if wrapped := repository.Save(ctx, &model); wrapped != nil {
		t.Fatalf("Save: %v", wrapped.Error)
	}

	if wrapped := repository.Remove(ctx, model); wrapped != nil {
		t.Fatalf("Remove: %v", wrapped.Error)
	}

	_, wrapped := repository.Get(ctx, model.ID)
	if wrapped == nil || wrapped.BaseError != exceptions.RecordNotFound {
		t.Errorf("Get after Remove: got %v, want RecordNotFound", wrapped)
	}
}

// Migrations must be idempotent across restarts: reopening an existing file is the
// normal path every time the container comes back up.
func TestReopeningExistingDatabaseSucceeds(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "reopen.db")
	migrations, _ := filepath.Abs("../../../scripts/sqlite/migrations")

	open := func() *sql.DB {
		db, err := utils.OpenDatabase(ctx, path, migrations)
		if err != nil {
			t.Fatalf("opening database: %v", err)
		}
		return db
	}

	first := open()
	model := entity.Model{Name: "persisted"}
	if wrapped := NewModelRepositoryWithDB(first).Save(ctx, &model); wrapped != nil {
		t.Fatalf("Save: %v", wrapped.Error)
	}
	first.Close()

	second := open()
	defer second.Close()

	loaded, wrapped := NewModelRepositoryWithDB(second).GetByName(ctx, "persisted")
	if wrapped != nil {
		t.Fatalf("GetByName after reopen: %v", wrapped.Error)
	}

	if loaded.ID != model.ID {
		t.Errorf("ID = %q, want %q", loaded.ID, model.ID)
	}
}
