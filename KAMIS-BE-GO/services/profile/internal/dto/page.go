package dto

import "github.com/pebintk/kamis-be-go/pkg/page"

// PageOf and NewPage are the shared pkg/page types, re-exported here so the
// profile call sites read as `dto.PageOf` / `dto.NewPage` — several of them take
// a parameter literally named `page`, which would shadow the package.
type PageOf[T any] = page.Of[T]

// NewPage assembles the Spring-shaped page metadata around one page of content.
func NewPage[T any](content []T, number, size int, total int64) PageOf[T] {
	return page.New(content, number, size, total)
}
