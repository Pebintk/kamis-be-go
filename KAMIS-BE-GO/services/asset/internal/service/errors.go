package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/karina/kamis-be-go/pkg/apierr"
	"github.com/karina/kamis-be-go/pkg/blob"
	"github.com/karina/kamis-be-go/pkg/database"
)

// notFound turns a repository miss into the service's 404, and passes any other
// error through untouched so a dropped connection is not reported as a missing
// asset.
func notFound(err error, platNomor string) error {
	if errors.Is(err, database.ErrNotFound) {
		return apierr.NotFoundf("Asset dengan plat nomor %s tidak ditemukan", platNomor)
	}
	return err
}

// photoKey derives the blob key for an asset's photo.
//
// Java used the plate number as the filename directly, which does not survive
// contact with a real plate: "B 1234 XYZ" contains spaces, and blob.ValidKey
// rejects it. Stripping the unsafe characters alone would let "B 1234" and
// "B-1234" collide onto one object, so a short digest of the original plate is
// appended to keep distinct assets on distinct keys.
func photoKey(platNomor, contentType string) string {
	var safe strings.Builder
	for i := 0; i < len(platNomor); i++ {
		c := platNomor[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			safe.WriteByte(c)
		}
	}

	sum := sha256.Sum256([]byte(platNomor))
	return safe.String() + "-" + hex.EncodeToString(sum[:4]) + blob.ExtensionFor(contentType)
}

// isUUID reports whether s is a canonical 8-4-4-4-12 hex UUID, standing in for
// the Java UUID.fromString calls without a UUID dependency.
func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i := 0; i < len(s); i++ {
		switch i {
		case 8, 13, 18, 23:
			if s[i] != '-' {
				return false
			}
		default:
			if !isHex(s[i]) {
				return false
			}
		}
	}
	return true
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}
