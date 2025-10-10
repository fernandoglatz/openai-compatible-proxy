package utils

import (
	"context"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/log"
	"fernandoglatz/openai-compatible-proxy/internal/infrastructure/config"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mongodb"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var MongoDatabase mongoDatabaseType

type mongoDatabaseType struct {
	Client      *mongo.Database
	collections map[string]*mongo.Collection
}

func ConnectToMongoDB(ctx context.Context) error {
	log.Info(ctx).Msg("Connecting to MongoDB")

	config := config.ApplicationConfig.Data.Mongo
	uri := config.Uri
	databaseName := config.Database

	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		return err
	}

	err = client.Ping(context.Background(), nil)
	if err != nil {
		return err
	}

	log.Info(ctx).Msg("Connected to MongoDB!")

	mongodbDriver, err := mongodb.WithInstance(client, &mongodb.Config{
		DatabaseName: databaseName,
	})
	if err != nil {
		return err
	}

	migrations, err := migrate.NewWithDatabaseInstance("file://scripts/mongo/migrations", databaseName, mongodbDriver)
	if err != nil {
		return err
	}

	err = migrations.Up()
	if err != nil && err != migrate.ErrNoChange {
		return err
	}

	MongoDatabase = mongoDatabaseType{
		Client:      client.Database(databaseName),
		collections: make(map[string]*mongo.Collection),
	}

	return nil
}

func (mongoDatabase mongoDatabaseType) GetCollection(collectionName string) *mongo.Collection {
	collection := mongoDatabase.collections[collectionName]

	if collection == nil {
		collection = mongoDatabase.Client.Collection(collectionName)
		mongoDatabase.collections[collectionName] = collection
	}

	return collection
}
