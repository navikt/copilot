package main

import (
	"fmt"
	"reflect"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"
)

// decodeBQRow maps a single BigQuery result row onto dst (a pointer to a struct),
// matching columns to struct fields by their `bigquery` tag.
//
// It is deliberately tolerant so that a single unexpected value never fails the
// whole request:
//   - NULL columns are left as the field's zero value.
//   - Numeric types are coerced between INTEGER and FLOAT (e.g. SUM() returning a
//     FLOAT into an int64 field, or an integer into a float64 field).
//   - Repeated columns decode into slices and nested RECORDs into structs.
//
// Columns with no matching struct field are ignored, and struct fields with no
// matching column are left untouched. This makes scanning resilient to BigQuery
// view/schema drift instead of crashing with "X is not assignable to Y".
func decodeBQRow(schema bigquery.Schema, row []bigquery.Value, dst any) error {
	rv := reflect.ValueOf(dst)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("decode target must be a non-nil pointer")
	}
	sv := rv.Elem()
	if sv.Kind() != reflect.Struct {
		return fmt.Errorf("decode target must point to a struct")
	}

	fieldByColumn := bqFieldIndex(sv.Type())
	for i, col := range schema {
		if i >= len(row) {
			break
		}
		fieldIdx, ok := fieldByColumn[col.Name]
		if !ok {
			continue
		}
		field := sv.Field(fieldIdx)
		if !field.CanSet() {
			continue
		}
		if err := assignBQField(field, row[i], col); err != nil {
			return fmt.Errorf("column %q: %w", col.Name, err)
		}
	}
	return nil
}

// bqFieldIndex maps a column name to the index of the struct field tagged with it.
func bqFieldIndex(t reflect.Type) map[string]int {
	out := make(map[string]int, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("bigquery")
		if tag == "" || tag == "-" {
			continue
		}
		// Strip any tag options such as `,nullable`.
		for j := 0; j < len(tag); j++ {
			if tag[j] == ',' {
				tag = tag[:j]
				break
			}
		}
		if tag != "" {
			out[tag] = i
		}
	}
	return out
}

func assignBQField(field reflect.Value, val bigquery.Value, fs *bigquery.FieldSchema) error {
	// NULL (or a missing repeated value) leaves the zero value in place.
	if val == nil {
		return nil
	}
	if fs != nil && fs.Repeated {
		return assignBQRepeated(field, val, fs)
	}
	if fs != nil && fs.Type == bigquery.RecordFieldType {
		return assignBQRecord(field, val, fs)
	}
	return assignBQScalar(field, val)
}

func assignBQRepeated(field reflect.Value, val bigquery.Value, fs *bigquery.FieldSchema) error {
	if field.Kind() != reflect.Slice {
		return fmt.Errorf("repeated column into non-slice field %s", field.Type())
	}
	items, ok := val.([]bigquery.Value)
	if !ok {
		return fmt.Errorf("repeated value is %T, expected []bigquery.Value", val)
	}
	slice := reflect.MakeSlice(field.Type(), len(items), len(items))
	elemSchema := &bigquery.FieldSchema{Type: fs.Type, Schema: fs.Schema}
	for i, item := range items {
		if err := assignBQField(slice.Index(i), item, elemSchema); err != nil {
			return fmt.Errorf("index %d: %w", i, err)
		}
	}
	field.Set(slice)
	return nil
}

func assignBQRecord(field reflect.Value, val bigquery.Value, fs *bigquery.FieldSchema) error {
	sub, ok := val.([]bigquery.Value)
	if !ok {
		return fmt.Errorf("record value is %T, expected []bigquery.Value", val)
	}
	if field.Kind() == reflect.Pointer {
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		field = field.Elem()
	}
	if field.Kind() != reflect.Struct {
		return fmt.Errorf("record column into non-struct field %s", field.Type())
	}
	return decodeBQRow(fs.Schema, sub, field.Addr().Interface())
}

func assignBQScalar(field reflect.Value, val bigquery.Value) error {
	// Fast path: directly assignable (string, civil.Date, time.Time, matching numeric).
	vv := reflect.ValueOf(val)
	if vv.Type().AssignableTo(field.Type()) {
		field.Set(vv)
		return nil
	}

	switch field.Kind() {
	case reflect.String:
		if s, ok := val.(string); ok {
			field.SetString(s)
			return nil
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch n := val.(type) {
		case int64:
			field.SetInt(n)
			return nil
		case int:
			field.SetInt(int64(n))
			return nil
		case float64:
			field.SetInt(int64(n))
			return nil
		case float32:
			field.SetInt(int64(n))
			return nil
		}
	case reflect.Float32, reflect.Float64:
		switch n := val.(type) {
		case float64:
			field.SetFloat(n)
			return nil
		case float32:
			field.SetFloat(float64(n))
			return nil
		case int64:
			field.SetFloat(float64(n))
			return nil
		case int:
			field.SetFloat(float64(n))
			return nil
		}
	case reflect.Bool:
		if b, ok := val.(bool); ok {
			field.SetBool(b)
			return nil
		}
	}
	return fmt.Errorf("cannot assign value of type %T to field of type %s", val, field.Type())
}

// readAllRows scans every row of the iterator into a slice of T using decodeBQRow.
func readAllRows[T any](it *bigquery.RowIterator) ([]T, error) {
	var results []T
	for {
		var row []bigquery.Value
		err := it.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("iterate results: %w", err)
		}
		var item T
		if err := decodeBQRow(it.Schema, row, &item); err != nil {
			return nil, fmt.Errorf("decode row: %w", err)
		}
		results = append(results, item)
	}
	return results, nil
}

// readSingleRow scans the first row of the iterator into a T, returning nil when
// the result set is empty.
func readSingleRow[T any](it *bigquery.RowIterator) (*T, error) {
	var row []bigquery.Value
	err := it.Next(&row)
	if err == iterator.Done {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read result: %w", err)
	}
	var item T
	if err := decodeBQRow(it.Schema, row, &item); err != nil {
		return nil, fmt.Errorf("decode row: %w", err)
	}
	return &item, nil
}
