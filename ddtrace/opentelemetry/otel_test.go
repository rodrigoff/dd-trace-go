// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023 Datadog, Inc.

package opentelemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func TestHttpDistributedTrace(t *testing.T) {
	tp, payloads, cleanup := mockTracerProvider(t)
	defer cleanup()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	tr := tp.Tracer("")

	sctx, rootSpan := tr.Start(context.Background(), "testRootSpan")

	w := otelhttp.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSpan := oteltrace.SpanFromContext(r.Context())
		assert.Equal(t, rootSpan.SpanContext().TraceID(), receivedSpan.SpanContext().TraceID())
	}), "testOperation")
	testServer := httptest.NewServer(w)
	defer testServer.Close()

	c := http.Client{Transport: otelhttp.NewTransport(nil)}
	req, err := http.NewRequestWithContext(sctx, http.MethodGet, testServer.URL, nil)
	require.NoError(t, err)
	resp, err := c.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close()) // Need to close body to cause otel span to end
	rootSpan.End()

	p := <-payloads
	numSpans := strings.Count(p, "\"span_id\"")
	assert.Equal(t, 3, numSpans)
	assert.Contains(t, p, "\"name\":\"testRootSpan\"")
	assert.Contains(t, p, "\"name\":\"testOperation\"")
	assert.Contains(t, p, "\"name\":\"HTTP GET\"")
}
