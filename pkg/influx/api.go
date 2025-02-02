package influx

import (
	"net/http"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/gorilla/mux"
	"github.com/grafana/mimir-proxies/pkg/remotewrite"
	"github.com/grafana/mimir-proxies/pkg/route"
	"github.com/grafana/mimir-proxies/pkg/server/middleware"
	"github.com/grafana/mimir/pkg/mimirpb"
	"github.com/weaveworks/common/user"
)

type API struct {
	logger              log.Logger
	client              remotewrite.Client
	recorder            Recorder
	maxRequestSizeBytes int
	maxSampleAgeSeconds int64
}

func (a *API) Register(router *mux.Router) {
	registerer := route.NewMuxRegisterer(router)

	// Registering two write endpoints; the second is necessary to allow for compatibility with clients that hard-code the endpoint
	registerer.RegisterRoute("/api/v1/push/influx/write", http.HandlerFunc(a.handleSeriesPush), http.MethodPost)
	registerer.RegisterRoute("/api/v2/write", http.HandlerFunc(a.handleSeriesPush), http.MethodPost)
}

func NewAPI(conf ProxyConfig, client remotewrite.Client, recorder Recorder) (*API, error) {
	return &API{
		logger:              conf.Logger,
		client:              client,
		recorder:            recorder,
		maxRequestSizeBytes: conf.MaxRequestSizeBytes,
		maxSampleAgeSeconds: conf.MaxSampleAgeSeconds,
	}, nil
}

// HandlerForInfluxLine is a http.Handler which accepts Influx Line protocol and converts it to WriteRequests.
func (a *API) handleSeriesPush(w http.ResponseWriter, r *http.Request) {
	logger := a.logger
	if traceID, ok := middleware.ExtractTraceID(r.Context()); ok {
		logger = log.With(logger, "traceID", traceID)
	}
	if orgID, err := user.ExtractOrgID(r.Context()); err == nil {
		logger = log.With(logger, "orgID", orgID)
	}
	if userID, err := user.ExtractUserID(r.Context()); err == nil {
		logger = log.With(logger, "userID", userID)
	}
	logger = log.With(logger, "path", r.URL.EscapedPath())

	beforeConversion := time.Now()

	ts, err, bytesRead := parseInfluxLineReader(r.Context(), r, a.maxRequestSizeBytes)
	logger = log.With(logger, "bytesRead", bytesRead)
	if err != nil {
		a.handleError(w, r, err, logger)
		return
	}

	nosMetrics := len(ts)
	logger = log.With(logger, "nosMetrics", nosMetrics)

	a.recorder.measureMetricsParsed(nosMetrics)
	a.recorder.measureConversionDuration(time.Since(beforeConversion))

	// Sigh, a write API optimisation needs me to jump through hoops.
	pts := make([]mimirpb.PreallocTimeseries, 0, nosMetrics)
	nowMs := time.Now().UnixMilli()
	nosDropped := 0
	for i := range ts {
		sampleTsMs := &ts[i].Samples[0].TimestampMs
		if a.maxSampleAgeSeconds > 0 && (nowMs-*sampleTsMs)/1000 > a.maxSampleAgeSeconds {
			nosDropped += 1
			_ = level.Info(logger).Log("msg", "dropped sample", "sample", ts[i].GoString())
		} else {
			pts = append(pts, mimirpb.PreallocTimeseries{
				TimeSeries: &ts[i],
			})
		}
	}
	if nosDropped > 0 {
		a.recorder.measureMetricsDropped(nosDropped)
	}
	rwReq := &mimirpb.WriteRequest{
		Timeseries: pts,
	}

	if err := a.client.Write(r.Context(), rwReq); err != nil {
		a.handleError(w, r, err, logger)
		return
	}
	a.recorder.measureMetricsWritten(len(rwReq.Timeseries))
	statusCode := http.StatusNoContent
	_ = level.Info(logger).Log("response_code", statusCode)
	w.WriteHeader(statusCode) // Needed for Telegraf, otherwise it tries to marshal JSON and considers the write a failure.
}
