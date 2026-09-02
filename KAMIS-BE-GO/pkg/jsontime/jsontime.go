// Package jsontime renders times the way the legacy Jackson DTOs did, so the
// frontend keeps parsing the same strings.
package jsontime

import (
	"bytes"
	"encoding/json"
	"strconv"
	"time"
)

// jakarta is the timezone the legacy @JsonFormat annotations pin dates to.
var jakarta = time.FixedZone("WIB", 7*60*60)

// javaDateLayout is how Jackson renders a java.util.Date once Spring Boot has
// disabled WRITE_DATES_AS_TIMESTAMPS: ISO-8601, milliseconds, numeric offset.
const javaDateLayout = "2006-01-02T15:04:05.000-07:00"

// Time serializes a time the way the Java DTOs do. The zone matters and is
// not uniform in the legacy code: the Client DTOs carry
// @JsonFormat(timezone="Asia/Jakarta") so they render as WIB, while the Supplier
// DTOs have no annotation and fall back to the ObjectMapper default of UTC.
// Construct with Jakarta or UTC accordingly.
type Time struct {
	time.Time
	loc *time.Location
}

// Jakarta renders t in Asia/Jakarta (the @JsonFormat-annotated fields).
func Jakarta(t time.Time) Time { return Time{Time: t, loc: jakarta} }

// UTC renders t in UTC (the un-annotated fields).
func UTC(t time.Time) Time { return Time{Time: t, loc: time.UTC} }

func (t Time) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte("null"), nil
	}
	loc := t.loc
	if loc == nil {
		loc = time.UTC
	}
	return []byte(strconv.Quote(t.Time.In(loc).Format(javaDateLayout))), nil
}

// UnmarshalJSON accepts what the other KAMIS services emit: an ISO-8601 string
// (with or without milliseconds) or a raw epoch-milliseconds number.
func (t *Time) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	if data[0] != '"' {
		ms, err := strconv.ParseInt(string(data), 10, 64)
		if err != nil {
			return err
		}
		t.Time = time.UnixMilli(ms)
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	for _, layout := range []string{javaDateLayout, time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, s); err == nil {
			t.Time = parsed
			return nil
		}
	}
	return &time.ParseError{Layout: javaDateLayout, Value: s}
}
