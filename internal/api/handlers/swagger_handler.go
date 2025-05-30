package handlers

import (
	"net/http"

	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
	httpSwagger "github.com/swaggo/http-swagger"
)

// SwaggerHandler handles Swagger UI and API documentation endpoints
type SwaggerHandler struct {
	logger logger.Logger
}

// NewSwaggerHandler creates a new SwaggerHandler instance
func NewSwaggerHandler(logger logger.Logger) *SwaggerHandler {
	return &SwaggerHandler{
		logger: logger,
	}
}

// GetSwaggerUI serves the Swagger UI interface
// @Summary Swagger UI
// @Description Interactive API documentation interface
// @Tags Documentation
// @Accept json
// @Produce text/html
// @Success 200 {string} string "Swagger UI HTML page"
// @Router /swagger/index.html [get]
func (h *SwaggerHandler) GetSwaggerUI(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Serving Swagger UI", logger.String("path", r.URL.Path))
	httpSwagger.WrapHandler(w, r)
}

// GetOpenAPISpec serves the OpenAPI specification
// @Summary OpenAPI specification
// @Description Returns the OpenAPI 3.0 specification in JSON format
// @Tags Documentation
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "OpenAPI specification"
// @Router /swagger/doc.json [get]
func (h *SwaggerHandler) GetOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Serving OpenAPI specification", logger.String("path", r.URL.Path))
	httpSwagger.WrapHandler(w, r)
}

// RedirectToSwagger redirects to the Swagger UI
func (h *SwaggerHandler) RedirectToSwagger(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Redirecting to Swagger UI")
	http.Redirect(w, r, "/swagger/index.html", http.StatusPermanentRedirect)
}

// GetAPIInfo provides basic API information
func (h *SwaggerHandler) GetAPIInfo(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Serving API info")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	apiInfo := `{
		"name": "GlobeCo Portfolio Accounting Service API",
		"version": "1.0",
		"description": "Financial transaction processing and portfolio balance management microservice",
		"documentation": {
			"swagger_ui": "/swagger/index.html",
			"openapi_spec": "/swagger/doc.json",
			"redoc": "/redoc"
		},
		"contact": {
			"name": "GlobeCo Support",
			"email": "noah@kasbench.org",
			"url": "https://github.com/kasbench/globeco-portfolio-accounting-service"
		},
		"license": {
			"name": "MIT",
			"url": "https://opensource.org/licenses/MIT"
		}
	}`

	_, _ = w.Write([]byte(apiInfo))
}
