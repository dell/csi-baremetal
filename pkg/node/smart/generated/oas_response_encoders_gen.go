// Code generated by ogen, DO NOT EDIT.

package api

import (
	"net/http"

	"github.com/go-faster/errors"
	"github.com/go-faster/jx"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func encodeGetAllDrivesSmartInfoResponse(response GetAllDrivesSmartInfoRes, w http.ResponseWriter, span trace.Span) error {
	switch response := response.(type) {
	case *SmartMetrics:
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(200)
		span.SetStatus(codes.Ok, http.StatusText(200))

		e := new(jx.Encoder)
		response.Encode(e)
		if _, err := e.WriteTo(w); err != nil {
			return errors.Wrap(err, "write")
		}

		return nil

	case *GetAllDrivesSmartInfoNotFound:
		w.WriteHeader(404)
		span.SetStatus(codes.Error, http.StatusText(404))

		return nil

	default:
		return errors.Errorf("unexpected response type: %T", response)
	}
}

func encodeGetSmartInfoResponse(response GetSmartInfoRes, w http.ResponseWriter, span trace.Span) error {
	switch response := response.(type) {
	case *SmartMetrics:
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(200)
		span.SetStatus(codes.Ok, http.StatusText(200))

		e := new(jx.Encoder)
		response.Encode(e)
		if _, err := e.WriteTo(w); err != nil {
			return errors.Wrap(err, "write")
		}

		return nil

	case *GetSmartInfoNotFound:
		w.WriteHeader(404)
		span.SetStatus(codes.Error, http.StatusText(404))

		return nil

	default:
		return errors.Errorf("unexpected response type: %T", response)
	}
}