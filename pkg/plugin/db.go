package plugin

import (
	"context"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func NewDBClient(uri string) (*mongo.Client, error) {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (d *Datasource) find(collectionName string, from time.Time, to time.Time, filter, projection string, maxPoints int64) ([]bson.M, error) {
	collection := d.db.Collection(collectionName)

	// Replace ' with " so that writing the filters isn't such a pain
	var parsedFilter bson.D
	if err := bson.UnmarshalExtJSON([]byte(strings.Replace(filter, "'", "\"", -1)), true, &parsedFilter); err != nil {
		return nil, err
	}

	fFilter := bson.D{
		{Key: "$and",
			Value: bson.A{
				bson.D{{Key: "timestamp", Value: bson.D{{Key: "$gte", Value: from}}}},
				bson.D{{Key: "timestamp", Value: bson.D{{Key: "$lte", Value: to}}}},
			},
		},
	}

	for _, e := range parsedFilter {
		fFilter = append(fFilter, e)
	}

	// We'll only return the selected field, order the results in ascending time, drop the _id and return a maximum of
	// the specified points.
	fProjection := options.Find().SetProjection(bson.D{{projection, 1}, {"timestamp", 1}, {"_id", 0}}).SetSort(bson.D{{"timestamp", 1}}).SetLimit(maxPoints)

	backend.Logger.Debug("final filter", "filter", fFilter)
	backend.Logger.Debug("final projection", "projection", fProjection)

	cursor, err := collection.Find(context.TODO(), fFilter, fProjection)
	if err != nil {
		return nil, err
	}

	var results []bson.M
	if err = cursor.All(context.TODO(), &results); err != nil {
		return nil, err
	}

	return results, nil
}
