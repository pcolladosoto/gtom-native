package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/data"

	"go.mongodb.org/mongo-driver/mongo"
)

// Make sure Datasource implements required interfaces. This is important to do
// since otherwise we will only get a not implemented error response from plugin in
// runtime. In this example datasource instance implements backend.QueryDataHandler,
// backend.CheckHealthHandler interfaces. Plugin should not implement all these
// interfaces - only those which are required for a particular task.
var (
	_ backend.QueryDataHandler      = (*Datasource)(nil)
	_ backend.CheckHealthHandler    = (*Datasource)(nil)
	_ instancemgmt.InstanceDisposer = (*Datasource)(nil)
)

// NewDatasource creates a new datasource instance.
// TODO: Implement MongoDB authentication by leveraging the HTTP-specific options maybe?
// TODO: We can also leverage settings.JSONData as hinted by the doc...
func NewDatasource(ctx context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	backend.Logger.Debug("creating a new datasource", "settings", settings)

	opts, err := settings.HTTPClientOptions(ctx)
	if err != nil {
		return nil, fmt.Errorf("http client options: %w", err)
	}

	cli, err := NewDBClient(settings.URL)
	if err != nil {
		return nil, fmt.Errorf("mongoclient new: %w", err)
	}

	if settings.BasicAuthEnabled {
		opts.BasicAuth.User = settings.BasicAuthUser
		opts.BasicAuth.Password = settings.DecryptedSecureJSONData["basicAuthPassword"]
	}

	return &Datasource{
		settings.URL,
		cli,
		cli.Database("telegrafData"),
	}, nil
}

// Datasource is an example datasource which can respond to data queries, reports
// its health and has streaming skills.
type Datasource struct {
	uri string
	cli *mongo.Client
	db  *mongo.Database
}

// Check https://grafana.com/developers/plugin-tools/create-a-plugin/extend-a-plugin/add-resource-handler
func (d *Datasource) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	switch req.Path {
	// case "metrics", "metric-payload-options", "variable", "tag-keys", "tag-values":
	case "metrics":
		backend.Logger.Debug("handling metrics request query", "body", req.Body)
		return handleMetrics(d.db, sender, req.Body)
	default:
		return sender.Send(&backend.CallResourceResponse{
			Status: http.StatusNotFound,
			Body:   []byte(fmt.Sprintf(`{"err": "requested non-existent resource %s"}`, req.Path)),
		})
	}
}

// Dispose here tells plugin SDK that plugin wants to clean up resources when a new instance
// created. As soon as datasource settings change detected by SDK old datasource instance will
// be disposed and a new one will be created using NewSampleDatasource factory function.
func (d *Datasource) Dispose() {
	// Clean up datasource instance resources.
	d.cli.Disconnect(context.TODO())
}

// QueryData handles multiple queries and returns multiple responses.
// req contains the queries []DataQuery (where each query contains RefID as a unique identifier).
// The QueryDataResponse contains a map of RefID to the response for each query, and each response
// contains Frames ([]*Frame).
func (d *Datasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	// create response struct
	response := backend.NewQueryDataResponse()

	// loop over queries and execute them individually.
	for _, q := range req.Queries {
		res := d.query(ctx, req.PluginContext, q)

		// save the response in a hashmap
		// based on with RefID as identifier
		response.Responses[q.RefID] = res
	}

	return response, nil
}

type queryModel struct{}

func (d *Datasource) query(_ context.Context, pCtx backend.PluginContext, query backend.DataQuery) backend.DataResponse {
	var response backend.DataResponse

	backend.Logger.Debug("making a query", "query", query, "pluginContext", pCtx)

	// Unmarshal the JSON into our queryModel.
	var qm queryModel

	err := json.Unmarshal(query.JSON, &qm)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("json unmarshal: %v", err.Error()))
	}

	// create data frame response.
	// For an overview on data frames and how grafana handles them:
	// https://grafana.com/developers/plugin-tools/introduction/data-frames
	frame := data.NewFrame("response")

	// add fields.
	frame.Fields = append(frame.Fields,
		data.NewField("time", nil, []time.Time{query.TimeRange.From, query.TimeRange.To}),
		data.NewField("values", nil, []int64{10, 20}),
	)

	// add the frames to the response.
	response.Frames = append(response.Frames, frame)

	return response
}

// CheckHealth handles health checks sent from Grafana to the plugin.
// The main use case for these health checks is the test button on the
// datasource configuration page which allows users to verify that
// a datasource is working as expected.
func (d *Datasource) CheckHealth(_ context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	backend.Logger.Debug("cheking the health of the backing mongo instance", "uri", d.uri)
	if err := d.cli.Ping(context.TODO(), nil); err != nil {
		backend.Logger.Error("error trying to ping the database", "err", err)
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: fmt.Sprintf("Error when pinging the database: %v", err),
		}, nil
	}

	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "Data source is working!",
	}, nil
}
