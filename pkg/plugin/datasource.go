package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/data"

	"go.mongodb.org/mongo-driver/bson/primitive"
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
		backend.Logger.Debug("answering query request", "q", q)

		res := d.query(ctx, req.PluginContext, q)

		// save the response in a hashmap
		// based on with RefID as identifier
		response.Responses[q.RefID] = res
	}

	return response, nil
}

// queryPayload defines the fields present in the payload embedded into queries.
type queryPayload struct {
	// FindQuery contains the find query to relay to the backing database.
	FindQuery string `json:"findQuery"`

	// Projection defines the filed to return.
	Projection string `json:"projection"`
}

// timeRange defines the format of the embedded time range in a request.
type timeRange struct {
	// The RFC3339-encoded time defining the start of the metrics window.
	From string `json:"from"`

	// The RFC3339-encoded time defining the end of the metrics window.
	To string `json:"to"`
}

type modeDiscriminator struct {
	// Editor mode defines hwo the query was built. It can be one of code or builder.
	// Depending on this mode, the Payload field will either be encoded as a string
	// or as a map[string]string.
	EditorMode string `json:"editorMode"`
}

// builderQueryModel defines the structure of the queries generated with the builder mode.
type builderQueryModel struct {
	// Target defines the metric (i.e. collection) being requested.
	Target string `json:"target"`

	// See the definition of the queryPayload struct
	Payload queryPayload `json:"payload"`

	// IntervalMs contains something we don't really know for now, but it sounds important!
	IntervalMs int `json:"intervalMs"`

	// MaxDataPoints defines the maximum number of points to return.
	MaxDataPoints int64 `json:"maxDataPoints"`

	// See the definition of the timeRange struct.
	TimeRange timeRange `json:"timeRange"`
}

// codeQueryModel defines the structure of the queries generated in code mode.
type codeQueryModel struct {
	// Target defines the metric (i.e. collection) being requested.
	Target string `json:"target"`

	// See the definition of the queryPayload struct
	Payload string `json:"payload"`

	// IntervalMs contains something we don't really know for now, but it sounds important!
	IntervalMs int `json:"intervalMs"`

	// MaxDataPoints defines the maximum number of points to return.
	MaxDataPoints int64 `json:"maxDataPoints"`

	// See the definition of the timeRange struct.
	TimeRange timeRange `json:"timeRange"`
}

func (d *Datasource) query(_ context.Context, pCtx backend.PluginContext, query backend.DataQuery) backend.DataResponse {
	var response backend.DataResponse

	backend.Logger.Debug("making a query", "query", query, "pluginContext", pCtx)

	// Unmarshal the JSON into our queryModel.
	modeTeller := modeDiscriminator{}
	if err := json.Unmarshal(query.JSON, &modeTeller); err != nil {
		backend.Logger.Error("error unmarshalling the mode teller", "err", err)
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("json unmarshal: %v", err.Error()))
	}

	backend.Logger.Debug("raw query", "query.JSON", fmt.Sprintf("%+v", string(query.JSON)))

	// TODO: The correct way of handling this mess is implementing a custom JSON unmarshaller, but who's got the time!
	qm := builderQueryModel{}
	if modeTeller.EditorMode == "builder" {
		if err := json.Unmarshal(query.JSON, &qm); err != nil {
			backend.Logger.Error("error unmarshalling the builder-mode query", "err", err)
			return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("json unmarshal: %v", err.Error()))
		}
	} else {
		qCode := codeQueryModel{}
		if err := json.Unmarshal(query.JSON, &qCode); err != nil {
			backend.Logger.Error("error unmarshalling the code-mode query", "err", err)
			return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("json unmarshal: %v", err.Error()))
		}

		qPayload := queryPayload{}
		if err := json.Unmarshal([]byte(qCode.Payload), &qPayload); err != nil {
			backend.Logger.Error("error unmarshalling the code-mode payload", "err", err)
			return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("json unmarshal: %v", err.Error()))
		}

		qm = builderQueryModel{
			Target:        qCode.Target,
			Payload:       qPayload,
			IntervalMs:    qCode.IntervalMs,
			MaxDataPoints: qCode.MaxDataPoints,
			TimeRange:     qCode.TimeRange,
		}
	}

	backend.Logger.Debug("parsed query", "qm", qm)

	results, err := d.find(qm.Target, query.TimeRange.From, query.TimeRange.To, qm.Payload.FindQuery, qm.Payload.Projection, query.MaxDataPoints)
	if err != nil {
		backend.Logger.Warn("error running the find() query", "err", err)
	}

	backend.Logger.Debug("query results", "results", fmt.Sprintf("%#v", results))

	if len(results) < 1 {
		return backend.ErrDataResponse(backend.StatusBadRequest, "got no fields back...")
	}

	timestampSlice := make([]time.Time, 0, len(results))
	sampleVal := results[0][qm.Payload.Projection]
	backend.Logger.Debug("sample value", "value", sampleVal)
	valueTypeFoo := reflect.TypeOf(sampleVal)
	valueType := reflect.TypeOf(&sampleVal)
	backend.Logger.Debug("detected value type", "valueType", valueType.String(), "kind", valueType.Kind())
	backend.Logger.Debug("detected value type", "valueTypeFoo", valueTypeFoo.String(), "kind", valueTypeFoo.Kind())

	valueSlice := reflect.MakeSlice(reflect.SliceOf(valueTypeFoo), 0, len(results))
	backend.Logger.Debug("value slize properties", "settable", valueSlice.CanSet(), "kind", valueSlice.Kind())

	settableSlice := reflect.New(valueSlice.Type())
	settableSlice.Elem().Set(valueSlice)
	backend.Logger.Debug("settable slize properties", "settable", settableSlice.CanSet(), "kind", settableSlice.Kind(), "type", settableSlice.Type().String(), "foo", settableSlice.Elem().Type().String(), "faa", settableSlice.Elem().CanSet())

	for i, res := range results {
		backend.Logger.Debug("result", "i", i, "data", fmt.Sprintf("%#v", res))
		for k, v := range res {
			backend.Logger.Debug("members", "k", k, "v", fmt.Sprintf("%#v, type: %T", v, v))
		}

		tStamp, ok := res["timestamp"].(primitive.DateTime)
		if !ok {
			continue
		}

		pValue, ok := res[qm.Payload.Projection]
		if !ok {
			continue
		}

		backend.Logger.Debug("appending to timestampSlice")
		timestampSlice = append(timestampSlice, tStamp.Time())

		val := settableSlice.Elem()
		// valueSlice = reflect.Append(valueSlice, reflect.ValueOf(pValue))
		val.Set(reflect.Append(val, reflect.ValueOf(pValue)))
		backend.Logger.Debug("appending to valueSlice", "value", pValue, "val", val.Type().String(), "len", val.Len())
	}

	backend.Logger.Debug("final timestamp slice", "timestampSlice", timestampSlice, "len", len(timestampSlice), "len(results)", len(results))
	backend.Logger.Debug("final value slice", "valueSlice", valueSlice)

	val := settableSlice.Elem()
	backend.Logger.Debug("final value slice", "settableSlice", settableSlice, "settableSlice.Elem()", val, "iface", val.Interface())

	// create data frame response.
	// For an overview on data frames and how grafana handles them:
	// https://grafana.com/developers/plugin-tools/introduction/data-frames
	frame := data.NewFrame("response")

	// add fields.
	frame.Fields = append(frame.Fields,
		data.NewField("time", nil, timestampSlice),
		data.NewField("values", nil, val.Interface()),
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
