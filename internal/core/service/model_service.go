package service

import (
	"context"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/exceptions"
	"fernandoglatz/openai-compatible-proxy/internal/core/entity"
	"fernandoglatz/openai-compatible-proxy/internal/core/port/repository"
)

type ModelService struct {
	repository repository.IModelRepository
}

func NewModelService(repository repository.IModelRepository) *ModelService {
	return &ModelService{
		repository: repository,
	}
}

func (service *ModelService) Get(ctx context.Context, id string) (entity.Model, *exceptions.WrappedError) {
	return service.repository.Get(ctx, id)
}

func (service *ModelService) GetByName(ctx context.Context, name string) (entity.Model, *exceptions.WrappedError) {
	return service.repository.GetByName(ctx, name)
}

func (service *ModelService) GetAll(ctx context.Context) ([]entity.Model, *exceptions.WrappedError) {
	return service.repository.GetAll(ctx)
}

func (service *ModelService) Save(ctx context.Context, model *entity.Model) *exceptions.WrappedError {
	return service.repository.Save(ctx, model)
}

func (service *ModelService) Remove(ctx context.Context, model entity.Model) *exceptions.WrappedError {
	return service.repository.Remove(ctx, model)
}
