package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/grafana/grafana-plugin-sdk-go/backend"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type (
	metricsRequest struct {
		Metric  string            `json:"metric"`
		Payload map[string]string `json:"payload"`
	}

	metricsReply struct {
		Label string `json:"label"`
		Value string `json:"value"`
	}
)

func handleMetrics(db *mongo.Database, sender backend.CallResourceResponseSender, body []byte) error {
	// Note we aren't using the request ATM for anything...
	req := metricsRequest{}
	if err := json.Unmarshal(body, &req); err != nil {
		backend.Logger.Error("couldn't unmarshal the metrics payload", "err", err)
		return sender.Send(&backend.CallResourceResponse{
			Status: http.StatusInternalServerError,
			Body:   []byte(fmt.Sprintf(`{"err": "couldn't unmarshal the payload: %v"}`, err)),
		})
	}

	// Bear in mind we could've used ListCollectionNames()!
	cursor, err := db.ListCollections(context.TODO(), bson.D{})
	if err != nil {
		backend.Logger.Error("couldn't retrieve the collections", "err", err)
		return sender.Send(&backend.CallResourceResponse{
			Status: http.StatusInternalServerError,
			Body:   []byte(fmt.Sprintf(`{"err": "couldn't retrieve the collections: %v"}`, err)),
		})
	}

	collections := []bson.M{}
	if err := cursor.All(context.TODO(), &collections); err != nil {
		backend.Logger.Error("couldn't unwrap the collections", "err", err)
		return sender.Send(&backend.CallResourceResponse{
			Status: http.StatusInternalServerError,
			Body:   []byte(fmt.Sprintf(`{"err": "couldn't unwrap the collections: %v"}`, err)),
		})
	}

	collectionNames := make([]metricsReply, 0, len(collections))
	for _, collection := range collections {
		cType, ok := collection["type"].(string)
		if !ok || cType != "timeseries" {
			continue
		}

		cName, ok := collection["name"].(string)
		if !ok {
			continue
		}

		collectionNames = append(collectionNames, metricsReply{cName, cName})
	}

	replyBody, err := json.Marshal(collectionNames)
	if err != nil {
		backend.Logger.Error("couldn't marshal the metrics", "err", err)
		return sender.Send(&backend.CallResourceResponse{
			Status: http.StatusInternalServerError,
			Body:   []byte(fmt.Sprintf(`{"err": "couldn't marshal the metrics: %v"}`, err)),
		})
	}

	return sender.Send(&backend.CallResourceResponse{
		Status: http.StatusOK,
		Body:   replyBody,
	})
}
