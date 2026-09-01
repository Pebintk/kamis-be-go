// Package router wires the resource routes and their auth rules — the Go analog
// of the service's WebSecurityConfig.
package router

import (
	"github.com/gin-gonic/gin"
	"github.com/karina/kamis-be-go/pkg/auth"
	"github.com/karina/kamis-be-go/pkg/httpx"
	"github.com/karina/kamis-be-go/services/resource/internal/handler"
)

// allRoles is Spring's hasAnyAuthority("Direksi","Finance","Operasional","Admin"),
// which the legacy config used both for the reads and as the /api/resource/**
// catch-all.
var allRoles = []string{"Admin", "Direksi", "Finance", "Operasional"}

// writeRoles guards everything that changes stock or catalogue data.
var writeRoles = []string{"Operasional", "Admin"}

func New(v *auth.Verifier, allowedOrigins []string, resources *handler.ResourceHandler) *gin.Engine {
	r := gin.New()
	// ForwardToken is installed even though this service makes no outbound
	// calls today, so that adding one does not require remembering to add the
	// middleware. It is the same global placement every service uses.
	r.Use(httpx.RequestLogger(), gin.Recovery(), httpx.CORS(allowedOrigins), auth.ForwardToken())

	r.GET("/health", handler.Health)

	// Nothing under /api is public here — unlike profile, this service issues no
	// tokens and has no registration endpoint.
	secured := r.Group("/api")
	secured.Use(v.GinAuth())
	{
		read := auth.GinRequireRole(allRoles...)
		write := auth.GinRequireRole(writeRoles...)

		secured.GET("/resource/viewall", read, resources.All)
		secured.GET("/resource/viewall/paginated", read, resources.Paginated)
		secured.GET("/resource/find/:idResource", read, resources.Detail)
		secured.GET("/resource/find-by-supplier/:idSupplier", read, resources.BySupplier)
		secured.GET("/resource/find-by-stock/:stock", read, resources.ByStock)

		// The legacy rule for /add is hasAnyAuthority("Admin","Operasional"),
		// the same pair as the writes below.
		secured.POST("/resource/add", write, resources.Add)
		secured.PUT("/resource/update/:idResource", write, resources.Update)
		secured.PUT("/resource/addToDb/:idResource/:stockUpdate", write, resources.AddToDB)
		secured.PUT("/resource/add-supplier", write, resources.AddSupplier)
		secured.PUT("/resource/update-supplier", write, resources.UpdateSupplier)
		secured.PUT("/resource/:idResource/add-stock", write, resources.AddStock)
		secured.PUT("/resource/:idResource/deduct-stock", write, resources.DeductStock)
	}

	return r
}
