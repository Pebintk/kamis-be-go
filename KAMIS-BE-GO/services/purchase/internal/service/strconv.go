package service

import "strconv"

// itoa keeps the int64-to-path-segment conversions in the cross-service URLs
// short at the call sites.
func itoa(v int64) string { return strconv.FormatInt(v, 10) }
