// Package router wires the project routes and their auth rules — the Go analog
// of the service's WebSecurityConfig.
package router

import (
	"github.com/gin-gonic/gin"
	"github.com/karina/kamis-be-go/pkg/auth"
	"github.com/karina/kamis-be-go/pkg/httpx"
	"github.com/karina/kamis-be-go/services/project/internal/handler"
)

// allRoles is the /api/project/** catch-all.
var allRoles = []string{"Admin", "Direksi", "Finance", "Operasional"}

func New(v *auth.Verifier, allowedOrigins []string, projects *handler.ProjectHandler) *gin.Engine {
	r := gin.New()
	r.Use(httpx.RequestLogger(), gin.Recovery(), httpx.CORS(allowedOrigins), auth.ForwardToken())

	r.GET("/health", handler.Health)

	// Nothing under /api is public: this service verifies tokens and issues none.
	secured := r.Group("/api")
	secured.Use(v.GinAuth())
	{
		read := auth.GinRequireRole(allRoles...)

		// Creating a project is Operasional alone — narrower than the other
		// services' writes, which admit Admin too.
		secured.POST("/project/add", auth.GinRequireRole("Operasional"), projects.Add)

		secured.GET("/project/all", read, projects.All)
		secured.GET("/project/all/paginated", read, projects.Paginated)
		secured.GET("/project/:id", read, projects.Detail)
	}

	return r
}
