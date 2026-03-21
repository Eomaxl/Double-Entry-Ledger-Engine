package docs

import (
	"embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed openapi.yaml swagger.html
var embeddedDocs embed.FS

// RegisterRoutes exposes OpenAPI and Swagger UI endpoints.
func RegisterRoutes(engine *gin.Engine) {
	engine.GET("/openapi.yaml", func(c *gin.Context) {
		data, err := embeddedDocs.ReadFile("openapi.yaml")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load OpenAPI spec"})
			return
		}
		c.Data(http.StatusOK, "application/yaml", data)
	})

	engine.GET("/docs", func(c *gin.Context) {
		data, err := embeddedDocs.ReadFile("swagger.html")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load Swagger UI"})
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})
}
