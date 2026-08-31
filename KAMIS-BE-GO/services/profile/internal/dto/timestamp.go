package dto

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

// Timestamp serializes a time the way the Java DTOs do. The zone matters and is
// not uniform in the legacy code: the Client DTOs carry
// @JsonFormat(timezone="Asia/Jakarta") so they render as WIB, while the Supplier
// DTOs have no annotation and fall back to the ObjectMapper default of UTC.
// Construct with JakartaTime or UTCTime accordingly.
type Timestamp struct {
	time.Time
	loc *time.Location
}

// JakartaTime renders t in Asia/Jakarta (the @JsonFormat-annotated fields).
func JakartaTime(t time.Time) Timestamp { return Timestamp{Time: t, loc: jakarta} }

// UTCTime renders t in UTC (the un-annotated fields).
func UTCTime(t time.Time) Timestamp { return Timestamp{Time: t, loc: time.UTC} }

func (t Timestamp) MarshalJSON() ([]byte, error) {
	if t.Time.IsZero() {
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
func (t *Timestamp) UnmarshalJSON(data []byte) error {
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
