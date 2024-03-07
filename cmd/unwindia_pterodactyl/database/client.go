package database

import (
	"context"
	"github.com/GSH-LAN/Unwindia_pterodactyl/cmd/unwindia_pterodactyl/environment"
	"github.com/kamva/mgm/v3"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

const (
	CollectionName = "pterodactyl_matchinfo"
	DatabaseName   = "unwindia"
	DefaultTimeout = 10 * time.Second
)

// DatabaseClient is the client-interface for the main mongodb database
type DatabaseClient interface {
	CreateJob(ctx context.Context, entry *Job) (primitive.ObjectID, error)
	UpdateJob(ctx context.Context, entry *Job) (primitive.ObjectID, error)
	// Get returns an existing DotlanForumStatus by the given id. Id is the id of the match within dotlan (tcontest.tcid)
	GetJob(ctx context.Context, id string) (*Job, error)
	List(ctx context.Context, filter interface{}) ([]*Job, error)
	GetLastJobForMatchId(ctx context.Context, id string, action Action) (*Job, error)
}

func NewClient(ctx context.Context, env *environment.Environment) (*DatabaseClientImpl, error) {
	client, err := mongo.NewClient(options.Client().ApplyURI(env.MongoDbURI))
	if err != nil {
		log.Error().Err(err)
		return nil, err
	}

	ctx, _ = context.WithTimeout(ctx, 10*time.Second)
	err = client.Connect(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Error creating default mongo connection")
		return nil, err
	}

	//return NewClientWithDatabase(ctx, client.Database(DatabaseName))

	err = mgm.SetDefaultConfig(nil, DatabaseName, options.Client().ApplyURI(env.MongoDbURI))
	if err != nil {
		log.Error().Err(err).Msg("Error creating mgm connection")
		return nil, err
	}

	dbClient := DatabaseClientImpl{
		ctx: ctx,
		//jobsCollection: db.Collection(CollectionName),
		jobsCollection:  mgm.Coll(&Job{}),
		matchCollection: client.Database(DatabaseName).Collection(CollectionName),
	}

	return &dbClient, err
}

type DatabaseClientImpl struct {
	ctx             context.Context
	jobsCollection  *mgm.Collection
	matchCollection *mongo.Collection
}

func (d DatabaseClientImpl) CreateJob(ctx context.Context, entry *Job) (primitive.ObjectID, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	entry.Status = JobStatusNew
	err := d.jobsCollection.CreateWithCtx(ctx, entry)

	return entry.ID, err
}

func (d DatabaseClientImpl) UpdateJob(ctx context.Context, entry *Job) (primitive.ObjectID, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	err := d.jobsCollection.UpdateWithCtx(ctx, entry)

	return entry.ID, err
}

func (d DatabaseClientImpl) GetJob(ctx context.Context, id string) (*Job, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	filter := bson.D{{"_id", id}}
	result := d.jobsCollection.FindOne(ctx, filter)
	if result.Err() != nil {
		return nil, result.Err()
	}

	var entry Job
	err := result.Decode(&entry)
	if err != nil {
		return nil, err
	}

	return &entry, nil
}

// func (d DatabaseClientImpl) List(ctx context.Context, filter interface{}, resultChan chan Result) {
func (d DatabaseClientImpl) List(ctx context.Context, filter interface{}) ([]*Job, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	if filter == nil {
		filter = bson.D{}
	}

	cur, err := d.jobsCollection.Find(ctx, filter)

	if err != nil {
		return nil, err
	}

	var jobs []*Job

	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var result Job
		if err := cur.Decode(&result); err != nil {
			log.Error().Err(err).Msg("Error decoding document")
		} else {
			jobs = append(jobs, &result)
		}
	}

	return jobs, nil
}

func (d DatabaseClientImpl) GetLastJobForMatchId(ctx context.Context, id string, action Action) (*Job, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	filter := bson.D{{"$and", []interface{}{
		bson.D{{"matchid", id}},
		bson.D{{"action", action}},
	}}}

	opts := options.FindOne().SetSort(bson.D{{"updatedAt", 1}})

	result := d.jobsCollection.FindOne(ctx, filter, opts)
	if result.Err() != nil {
		return nil, result.Err()
	}

	var entry Job
	err := result.Decode(&entry)
	if err != nil {
		return nil, err
	}

	return &entry, nil
}
