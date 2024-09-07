package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/grafana/grafana-plugin-sdk-go/backend"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type (
	metricsRequest struct {
		Metric  string            `json:"metric"`
		Payload map[string]string `json:"payload"`
	}

	metricsReply struct {
		Label   string           `json:"label,omitempty"`
		Value   string           `json:"value"`
		Payload []metricsPayload `json:"payload,omitempty"`
	}

	metricsPayload struct {
		Label        string       `json:"label,omitempty"`
		Name         string       `json:"name"`
		Type         string       `json:"type"`
		PlaceHolder  string       `json:"placeholder,omitempty"`
		ReloadMetric bool         `json:"reloadMetric,omitempty"`
		Width        int          `json:"width,omitempty"`
		Options      []labelValue `json:"options"`
	}

	labelValue struct {
		Label string `json:"label"`
		Value string `json:"value"`
	}

	payloadType int

	// metricPayloadOptions represents the body of requests to the /metric-payload-options
	// endpoint.
	metricPayloadOptions struct {
		// Metric beeing requested.
		Metric string `json:"metric"`

		// Currentñy configured payload
		Payload metricsPayload `json:"payload"`

		// Name of the payload whose options need to be obatained
		Name string `json:"name"`
	}

	metricTags map[string]string
)

const (
	selectPayload payloadType = iota
	multiSelectPayload
	inputPayload
	textAreaPayload
)

func (p payloadType) String() string {
	switch p {
	case selectPayload:
		return "select"
	case multiSelectPayload:
		return "multi-select"
	case inputPayload:
		return "input"
	case textAreaPayload:
		return "textarea"
	default:
		return "select"
	}
}

var tagsProjection = options.Find().SetProjection(bson.D{{"tags", 1}, {"_id", 0}}).SetSort(bson.D{{"timestamp", -1}}).SetLimit(1)

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

		col := db.Collection(cName)

		tagsCursor, err := col.Find(context.TODO(), bson.D{}, tagsProjection)
		if err != nil {
			backend.Logger.Error("couldn't get a cursor for the metric tags", "err", err)
			return sender.Send(&backend.CallResourceResponse{
				Status: http.StatusInternalServerError,
				Body:   []byte(fmt.Sprintf(`{"err": "couldn't get a cursor for the metrics tags: %v"}`, err)),
			})
		}

		tags := []bson.M{}
		if err := tagsCursor.All(context.TODO(), &tags); err != nil {
			backend.Logger.Error("couldn't decode the metric tags", "err", err)
			return sender.Send(&backend.CallResourceResponse{
				Status: http.StatusInternalServerError,
				Body:   []byte(fmt.Sprintf(`{"err": "couldn't decode the metrics tags: %v"}`, err)),
			})
		}

		if len(tags) <= 0 {
			backend.Logger.Error("didn't receive any document", "tags", tags)
			continue
		}

		backend.Logger.Debug("discovered tags", "tags", tags, "tags[0]['tags']", tags[0]["tags"], "type", fmt.Sprintf("%T", tags[0]["tags"]))

		tagMap, ok := tags[0]["tags"].(primitive.M)
		if !ok {
			backend.Logger.Debug("type assertion for tags[0]['tags'] didn't work...")
			continue
		}

		keys := make([]string, 0, len(tagMap))
		for k := range tagMap {
			keys = append(keys, k)
		}

		backend.Logger.Debug("discovered keys", "keys", keys)

		payloads := []metricsPayload{}
		for _, tag := range keys {
			disResults, err := col.Distinct(context.TODO(), "tags."+tag, bson.D{})
			if err != nil {
				continue
			}

			backend.Logger.Debug("distinct values for tag", "tag", tag, "vals", disResults)

			options := []labelValue{}
			for _, disResult := range disResults {
				switch disResultParsed := disResult.(type) {
				case map[string]string:
					for k := range disResultParsed {
						options = append(options, labelValue{k, k})
					}
				case string:
					options = append(options, labelValue{disResultParsed, disResultParsed})
				default:
					backend.Logger.Debug("didn't get a type for disResult", "disResult", disResult, "type", fmt.Sprintf("%T", disResult))
				}
			}

			payloads = append(payloads, metricsPayload{
				Label:       tag,
				Name:        tag,
				Type:        selectPayload.String(),
				PlaceHolder: "miau",
				Options:     options,
			})
		}

		collectionNames = append(collectionNames, metricsReply{cName, cName, payloads})
	}

	replyBody, err := json.Marshal(collectionNames)
	if err != nil {
		backend.Logger.Error("couldn't marshal the metrics", "err", err)
		return sender.Send(&backend.CallResourceResponse{
			Status: http.StatusInternalServerError,
			Body:   []byte(fmt.Sprintf(`{"err": "couldn't marshal the metrics: %v"}`, err)),
		})
	}

	backend.Logger.Debug("replying to metrics request", "payload", string(replyBody))

	return sender.Send(&backend.CallResourceResponse{
		Status: http.StatusOK,
		Body:   replyBody,
	})
}
