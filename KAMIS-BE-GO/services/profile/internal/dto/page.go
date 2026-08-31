package dto

import "math"

// PageOf mirrors the subset of Spring's Page JSON that the frontend consumes.
type PageOf[T any] struct {
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

// NewPage assembles the Spring-shaped page metadata around one page of content.
func NewPage[T any](content []T, page, size int, total int64) PageOf[T] {
	totalPages := 0
	if size > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(size)))
	}
	return PageOf[T]{
		Content:          content,
		Number:           page,
		Size:             size,
		TotalElements:    total,
		TotalPages:       totalPages,
		First:            page == 0,
		Last:             page >= totalPages-1,
		NumberOfElements: len(content),
		Empty:            len(content) == 0,
	}
}
