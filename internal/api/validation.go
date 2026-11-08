package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"reflect"
	"strings"
)

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("write JSON response: %v", err)
	}
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return decodeError(err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return invalidInput("", "one JSON object", "request must contain one JSON object")
		}
		return decodeError(err)
	}
	return nil
}

// decodeErrorは、JSONのデコード失敗に項目名と期待する型を添える。
func decodeError(err error) error {
	var typeError *json.UnmarshalTypeError
	if errors.As(err, &typeError) && typeError.Field != "" {
		return &inputError{field: typeError.Field, expected: jsonTypeName(typeError.Type), err: err}
	}
	if field, ok := strings.CutPrefix(err.Error(), "json: unknown field "); ok {
		return &inputError{field: strings.Trim(field, `"`), expected: "a documented field", err: err}
	}
	return err
}

func jsonTypeName(value reflect.Type) string {
	switch value.Kind() {
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "number"
	case reflect.String:
		return "string"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Map, reflect.Struct:
		return "object"
	}
	return value.String()
}

func jsonDecodeStatus(err error) int {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusBadRequest
}

// jsonRouterErrorsは、ルーターが返す平文の404・405をAPIのエラー形式にそろえる。
func jsonRouterErrors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writer := &routerErrorWriter{ResponseWriter: w}
		next.ServeHTTP(writer, r)
		writer.finish(r)
	})
}

type routerErrorWriter struct {
	http.ResponseWriter
	status  int
	replace bool
}

func (w *routerErrorWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	// ルーターの平文応答だけを差し替える。APIのJSON応答はそのまま通す。
	if (status == http.StatusNotFound || status == http.StatusMethodNotAllowed) && strings.HasPrefix(w.Header().Get("Content-Type"), "text/plain") {
		w.replace = true
		return
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *routerErrorWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if w.replace {
		return len(data), nil
	}
	return w.ResponseWriter.Write(data)
}

func (w *routerErrorWriter) finish(r *http.Request) {
	if !w.replace {
		return
	}
	err := fmt.Errorf("no route for %s %s", r.Method, r.URL.Path)
	if w.status == http.StatusMethodNotAllowed {
		err = fmt.Errorf("method %s is not allowed for %s", r.Method, r.URL.Path)
	}
	writeAPIError(w.ResponseWriter, w.status, err)
}
