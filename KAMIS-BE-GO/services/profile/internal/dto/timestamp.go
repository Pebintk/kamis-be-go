package dto

import (
	"time"

	"github.com/pebintk/kamis-be-go/pkg/jsontime"
)

// Timestamp is the shared pkg/jsontime type, re-exported under the name the
// profile DTOs already use. Build one with JakartaTime or UTCTime: the zone is
// not uniform in the legacy code, since the Client DTOs carry
// @JsonFormat(timezone="Asia/Jakarta") while the Supplier DTOs have no
// annotation and fall back to UTC.
type Timestamp = jsontime.Time

// JakartaTime renders t in Asia/Jakarta (the @JsonFormat-annotated fields).
func JakartaTime(t time.Time) Timestamp { return jsontime.Jakarta(t) }

// UTCTime renders t in UTC (the un-annotated fields).
func UTCTime(t time.Time) Timestamp { return jsontime.UTC(t) }
