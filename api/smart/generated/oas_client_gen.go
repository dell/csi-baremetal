// Code generated by ogen, DO NOT EDIT.

package api

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/go-faster/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.19.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/ogen-go/ogen/conv"
	ht "github.com/ogen-go/ogen/http"
	"github.com/ogen-go/ogen/otelogen"
	"github.com/ogen-go/ogen/uri"
)

// Invoker invokes operations described by OpenAPI v3 specification.
type Invoker interface {
	// GetAllDrivesSmartInfo invokes get-all-drives-smart-info operation.
	//
	// Retrieve all discovered disks information/metrics.
	//
	// GET /smart/api/v1/drives
	GetAllDrivesSmartInfo(ctx context.Context) (GetAllDrivesSmartInfoRes, error)
	// GetDriveSmartInfo invokes get-drive-smart-info operation.
	//
	// Retrieve the disk information/metrics with the matching serial number.
	//
	// GET /smart/api/v1/drive/{serialNumber}
	GetDriveSmartInfo(ctx context.Context, params GetDriveSmartInfoParams) (GetDriveSmartInfoRes, error)
}

// Client implements OAS client.
type Client struct {
	serverURL *url.URL
	baseClient
}

var _ Handler = struct {
	*Client
}{}

func trimTrailingSlashes(u *url.URL) {
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath = strings.TrimRight(u.RawPath, "/")
}

// NewClient initializes new Client defined by OAS.
func NewClient(serverURL string, opts ...ClientOption) (*Client, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return nil, err
	}
	trimTrailingSlashes(u)

	c, err := newClientConfig(opts...).baseClient()
	if err != nil {
		return nil, err
	}
	return &Client{
		serverURL:  u,
		baseClient: c,
	}, nil
}

type serverURLKey struct{}

// WithServerURL sets context key to override server URL.
func WithServerURL(ctx context.Context, u *url.URL) context.Context {
	return context.WithValue(ctx, serverURLKey{}, u)
}

func (c *Client) requestURL(ctx context.Context) *url.URL {
	u, ok := ctx.Value(serverURLKey{}).(*url.URL)
	if !ok {
		return c.serverURL
	}
	return u
}

// GetAllDrivesSmartInfo invokes get-all-drives-smart-info operation.
//
// Retrieve all discovered disks information/metrics.
//
// GET /smart/api/v1/drives
func (c *Client) GetAllDrivesSmartInfo(ctx context.Context) (GetAllDrivesSmartInfoRes, error) {
	res, err := c.sendGetAllDrivesSmartInfo(ctx)
	return res, err
}

func (c *Client) sendGetAllDrivesSmartInfo(ctx context.Context) (res GetAllDrivesSmartInfoRes, err error) {
	otelAttrs := []attribute.KeyValue{
		otelogen.OperationID("get-all-drives-smart-info"),
		semconv.HTTPMethodKey.String("GET"),
		semconv.HTTPRouteKey.String("/smart/api/v1/drives"),
	}

	// Run stopwatch.
	startTime := time.Now()
	defer func() {
		// Use floating point division here for higher precision (instead of Millisecond method).
		elapsedDuration := time.Since(startTime)
		c.duration.Record(ctx, float64(float64(elapsedDuration)/float64(time.Millisecond)), metric.WithAttributes(otelAttrs...))
	}()

	// Increment request counter.
	c.requests.Add(ctx, 1, metric.WithAttributes(otelAttrs...))

	// Start a span for this request.
	ctx, span := c.cfg.Tracer.Start(ctx, "GetAllDrivesSmartInfo",
		trace.WithAttributes(otelAttrs...),
		clientSpanKind,
	)
	// Track stage for error reporting.
	var stage string
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, stage)
			c.errors.Add(ctx, 1, metric.WithAttributes(otelAttrs...))
		}
		span.End()
	}()

	stage = "BuildURL"
	u := uri.Clone(c.requestURL(ctx))
	var pathParts [1]string
	pathParts[0] = "/smart/api/v1/drives"
	uri.AddPathParts(u, pathParts[:]...)

	stage = "EncodeRequest"
	r, err := ht.NewRequest(ctx, "GET", u)
	if err != nil {
		return res, errors.Wrap(err, "create request")
	}

	stage = "SendRequest"
	resp, err := c.cfg.Client.Do(r)
	if err != nil {
		return res, errors.Wrap(err, "do request")
	}
	defer resp.Body.Close()

	stage = "DecodeResponse"
	result, err := decodeGetAllDrivesSmartInfoResponse(resp)
	if err != nil {
		return res, errors.Wrap(err, "decode response")
	}

	return result, nil
}

// GetDriveSmartInfo invokes get-drive-smart-info operation.
//
// Retrieve the disk information/metrics with the matching serial number.
//
// GET /smart/api/v1/drive/{serialNumber}
func (c *Client) GetDriveSmartInfo(ctx context.Context, params GetDriveSmartInfoParams) (GetDriveSmartInfoRes, error) {
	res, err := c.sendGetDriveSmartInfo(ctx, params)
	return res, err
}

func (c *Client) sendGetDriveSmartInfo(ctx context.Context, params GetDriveSmartInfoParams) (res GetDriveSmartInfoRes, err error) {
	otelAttrs := []attribute.KeyValue{
		otelogen.OperationID("get-drive-smart-info"),
		semconv.HTTPMethodKey.String("GET"),
		semconv.HTTPRouteKey.String("/smart/api/v1/drive/{serialNumber}"),
	}

	// Run stopwatch.
	startTime := time.Now()
	defer func() {
		// Use floating point division here for higher precision (instead of Millisecond method).
		elapsedDuration := time.Since(startTime)
		c.duration.Record(ctx, float64(float64(elapsedDuration)/float64(time.Millisecond)), metric.WithAttributes(otelAttrs...))
	}()

	// Increment request counter.
	c.requests.Add(ctx, 1, metric.WithAttributes(otelAttrs...))

	// Start a span for this request.
	ctx, span := c.cfg.Tracer.Start(ctx, "GetDriveSmartInfo",
		trace.WithAttributes(otelAttrs...),
		clientSpanKind,
	)
	// Track stage for error reporting.
	var stage string
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, stage)
			c.errors.Add(ctx, 1, metric.WithAttributes(otelAttrs...))
		}
		span.End()
	}()

	stage = "BuildURL"
	u := uri.Clone(c.requestURL(ctx))
	var pathParts [2]string
	pathParts[0] = "/smart/api/v1/drive/"
	{
		// Encode "serialNumber" parameter.
		e := uri.NewPathEncoder(uri.PathEncoderConfig{
			Param:   "serialNumber",
			Style:   uri.PathStyleSimple,
			Explode: false,
		})
		if err := func() error {
			return e.EncodeValue(conv.StringToString(params.SerialNumber))
		}(); err != nil {
			return res, errors.Wrap(err, "encode path")
		}
		encoded, err := e.Result()
		if err != nil {
			return res, errors.Wrap(err, "encode path")
		}
		pathParts[1] = encoded
	}
	uri.AddPathParts(u, pathParts[:]...)

	stage = "EncodeRequest"
	r, err := ht.NewRequest(ctx, "GET", u)
	if err != nil {
		return res, errors.Wrap(err, "create request")
	}

	stage = "SendRequest"
	resp, err := c.cfg.Client.Do(r)
	if err != nil {
		return res, errors.Wrap(err, "do request")
	}
	defer resp.Body.Close()

	stage = "DecodeResponse"
	result, err := decodeGetDriveSmartInfoResponse(resp)
	if err != nil {
		return res, errors.Wrap(err, "decode response")
	}

	return result, nil
}