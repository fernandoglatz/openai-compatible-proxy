package repository

import (
	"context"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/exceptions"
	"fernandoglatz/openai-compatible-proxy/internal/core/entity"
)

type IModelRepository interface {
	Get(ctx context.Context, id string) (entity.Model, *exceptions.WrappedError)
	GetByName(ctx context.Context, name string) (entity.Model, *exceptions.WrappedError)
	GetAll(ctx context.Context) ([]entity.Model, *exceptions.WrappedError)
	Save(ctx context.Context, redirect *entity.Model) *exceptions.WrappedError
	Remove(ctx context.Context, redirect entity.Model) *exceptions.WrappedError
}
