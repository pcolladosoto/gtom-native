package plugin

import (
	"context"

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

// func (d *db) find(collectionName, from, to, filter string) ([]byte, error) {
// 	collection, ok := d.collections[collectionName]
// 	if !ok {
// 		slog.Debug("creating handle for collection", "collection", collectionName)
// 		collection = d.db.Collection(collectionName)
// 		d.collections[collectionName] = collection
// 	}
//
// 	// Replace ' with " so that writing the filters isn't such a pain
// 	var parsedFilter bson.D
// 	if err := bson.UnmarshalExtJSON([]byte(strings.Replace(filter, "'", "\"", -1)), true, &parsedFilter); err != nil {
// 		return nil, err
// 	}
//
// 	fromParsed, err := time.Parse(time.RFC3339, from)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	toParsed, err := time.Parse(time.RFC3339, to)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	fFilter := bson.D{
// 		{Key: "$and",
// 			Value: bson.A{
// 				bson.D{{Key: "timestamp", Value: bson.D{{Key: "$gte", Value: fromParsed}}}},
// 				bson.D{{Key: "timestamp", Value: bson.D{{Key: "$lte", Value: toParsed}}}},
// 			},
// 		},
// 	}
//
// 	for _, e := range parsedFilter {
// 		fFilter = append(fFilter, e)
// 	}
//
// 	slog.Debug("final filter", "filter", fFilter)
//
// 	cursor, err := collection.Find(context.TODO(), fFilter)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	var results []bson.M
// 	if err = cursor.All(context.TODO(), &results); err != nil {
// 		return nil, err
// 	}
//
// 	mResults, err := json.Marshal(results)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	return mResults, nil
// }
