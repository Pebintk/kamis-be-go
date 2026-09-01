// Package page reproduces the slice of Spring Data's Page JSON that the KAMIS
// frontend consumes. Every service's paginated endpoint returns this shape, so
// it lives here rather than in each service's dto package.
package page

import "math"

// Of is one page of content plus the metadata Spring emits alongside it.
type Of[T any] struct {
	Content          []T   `json:"content"`
	Number           int   `json:"number"`
	Size             int   `json:"size"`
	TotalElements    int64 `json:"totalElements"`
	TotalPages       int   `json:"totalPages"`
	First            bool  `json:"first"`
	Last             bool  `json:"last"`
	NumberOfElements int   `json:"numberOfElements"`
	Empty            bool  `json:"empty"`
}

// New assembles the Spring-shaped page metadata around one page of content.
// number is zero-based, matching Spring's Pageable.
func New[T any](content []T, number, size int, total int64) Of[T] {
	totalPages := 0
	if size > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(size)))
	}
	return Of[T]{
		Content:          content,
		Number:           number,
		Size:             size,
		TotalElements:    total,
		TotalPages:       totalPages,
		First:            number == 0,
		Last:             number >= totalPages-1,
		NumberOfElements: len(content),
		Empty:            len(content) == 0,
	}
}
