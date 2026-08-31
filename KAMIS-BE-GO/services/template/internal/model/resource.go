// Package model holds the GORM entities for this service (the JPA @Entity layer).
package model

import "time"

// Resource is a sample entity demonstrating the model layer. Replace it with the
// real domain entities when porting an actual service.
type Resource struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"not null" json:"name"`
	Quantity  int       `gorm:"not null;default:0" json:"quantity"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
