package repository

import (
	"context"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/constants"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/exceptions"
	"fernandoglatz/openai-compatible-proxy/internal/core/entity"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type ModelRepository struct {
	collection *mongo.Collection
}

func NewModelRepository() *ModelRepository {
	return &ModelRepository{
		collection: utils.MongoDatabase.GetCollection("models"),
	}
}

func (repository *ModelRepository) Get(ctx context.Context, id string) (entity.Model, *exceptions.WrappedError) {
	filter := bson.M{"id": id}
	return repository.getByFilter(ctx, filter)
}

func (repository *ModelRepository) GetByName(ctx context.Context, name string) (entity.Model, *exceptions.WrappedError) {
	filter := bson.M{"name": name}
	return repository.getByFilter(ctx, filter)
}

func (repository *ModelRepository) getByFilter(ctx context.Context, filter interface{}) (entity.Model, *exceptions.WrappedError) {
	var model entity.Model

	err := repository.collection.FindOne(ctx, filter).Decode(&model)
	if err == mongo.ErrNoDocuments {
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

	cursor, err := repository.collection.Find(ctx, bson.D{})
	if err != nil {
		return models, &exceptions.WrappedError{
			Error: err,
		}
	}

	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var model entity.Model
		err = cursor.Decode(&model)
		if err != nil {
			return models, &exceptions.WrappedError{
				Error: err,
			}
		}

		repository.correctTimezone(&model)
		models = append(models, model)
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

		_, err := repository.collection.InsertOne(ctx, model)
		if err != nil {
			return &exceptions.WrappedError{
				Error: err,
			}
		}

	} else {
		filter := bson.M{"id": model.ID}
		_, err := repository.collection.ReplaceOne(ctx, filter, model)
		if err != nil {
			return &exceptions.WrappedError{
				Error: err,
			}
		}
	}

	return nil
}

func (repository *ModelRepository) Remove(ctx context.Context, model entity.Model) *exceptions.WrappedError {
	filter := bson.M{"id": model.ID}
	_, err := repository.collection.DeleteOne(ctx, filter)
	if err != nil {
		return &exceptions.WrappedError{
			Error: err,
		}
	}

	return nil
}

func (repository *ModelRepository) correctTimezone(model *entity.Model) {
	location, _ := time.LoadLocation(utils.GetTimezone())
	model.CreatedAt = model.CreatedAt.In(location)
	model.UpdatedAt = model.UpdatedAt.In(location)
}
