package web_test

import (
	"flag"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "github.com/tomekjarosik/qivitals/gen/api/qivitals/v1"
	"github.com/tomekjarosik/qivitals/internal/web"
	"github.com/tomekjarosik/qivitals/internal/web/handlers"
)

var update = flag.Bool("update", false, "update golden files")

var timestampRegex = regexp.MustCompile(`data-ts="\d+"`)
var plainTimestampRegex = regexp.MustCompile(`\b\d{10}\b`)

func normalizeTimestamps(b []byte) []byte {
	s := string(b)
	s = timestampRegex.ReplaceAllString(s, `data-ts="0"`)
	s = plainTimestampRegex.ReplaceAllString(s, "0")
	return []byte(s)
}

// testEnv wraps everything needed to run a sensor detail test.
type testEnv struct {
	client   v1.QiVitalsServiceClient
	renderer web.Renderer
	mux      *http.ServeMux
}

// envOption is a functional option for building a testEnv.
type envOption func(*envConfig)

type envConfig struct {
	id   string
	name string
	ns   string
	lbl  map[string]string
	spec *v1.SensorSpec
	data map[string]string
}

func defaultConfig() *envConfig {
	return &envConfig{
		id:   "test-sensor-default",
		name: "Default Sensor",
		ns:   "default",
		spec: &v1.SensorSpec{
			GracefulPeriodSeconds: 60,
			FailurePeriodSeconds:  120,
			Rules:                 []*v1.ConditionRule{},
		},
		data: map[string]string{
			"battery": "95",
			"cpu":     "42",
		},
	}
}

func newTestEnv(t *testing.T, opts ...envOption) *testEnv {
	t.Helper()

	cfg := defaultConfig()
	for _, apply := range opts {
		apply(cfg)
	}

	client, _ := getRealQiVitalsClient(t)

	_, err := client.RegisterSensor(t.Context(), &v1.RegisterSensorRequest{
		Sensor: &v1.Sensor{
			Metadata: &v1.ObjectMeta{
				Id:        cfg.id,
				Name:      cfg.name,
				Namespace: cfg.ns,
				Labels:    cfg.lbl,
			},
			Spec: cfg.spec,
		},
	})
	require.NoError(t, err)

	if cfg.data != nil && len(cfg.data) > 0 {
		_, err = client.ReportSensor(t.Context(), &v1.ReportSensorRequest{
			Id:   cfg.id,
			Data: cfg.data,
		})
		require.NoError(t, err)
	}

	renderer := web.NewTemplateRenderer()
	handler := handlers.NewSensorDetailsHandler(renderer, client)

	mux := http.NewServeMux()
	mux.Handle("GET /sensors/{id}", handler)

	return &testEnv{
		client:   client,
		renderer: renderer,
		mux:      mux,
	}
}

func (e *testEnv) doRequest(t *testing.T, url string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	e.mux.ServeHTTP(w, req)
	return w
}

// --- Options ---

func withID(id string) envOption {
	return func(c *envConfig) { c.id = id }
}

func withName(name string) envOption {
	return func(c *envConfig) { c.name = name }
}

func withNamespace(ns string) envOption {
	return func(c *envConfig) { c.ns = ns }
}

func withLabels(lbls map[string]string) envOption {
	return func(c *envConfig) { c.lbl = lbls }
}

func withConditionRules(rules []*v1.ConditionRule) envOption {
	return func(c *envConfig) { c.spec.Rules = rules }
}

func withData(data map[string]string) envOption {
	return func(c *envConfig) { c.data = data }
}

func withNoData(_ *envConfig) {
}
