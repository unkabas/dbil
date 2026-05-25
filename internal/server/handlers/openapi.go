package handlers

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.yaml
var openapiSpec []byte

// OpenAPIHandler — GET /api/openapi.yaml. Returns the embedded spec.
// Unauthed by design: clients (curl, Swagger UI, oapi-codegen, CI) need to
// read the contract before they can authenticate. The protected endpoints
// themselves still require a bearer token.
func OpenAPIHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=60")
		_, _ = w.Write(openapiSpec)
	}
}

// SwaggerUIHandler — GET /api/docs. Serves a small HTML page that loads
// the Swagger UI bundle from unpkg and points it at /api/openapi.yaml.
// Air-gapped deployments can replace this with an embedded copy of the
// swagger-ui dist (~3 MB).
func SwaggerUIHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300")
		_, _ = w.Write([]byte(swaggerHTML))
	}
}

const swaggerHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <title>DBil API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  <style>
    body { margin: 0; background: #0d0f12; }
    .swagger-ui .topbar { display: none; }
  </style>
</head>
<body>
  <div id="swagger"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({
      url: '/api/openapi.yaml',
      dom_id: '#swagger',
      deepLinking: true,
      persistAuthorization: true,
    });
  </script>
</body>
</html>
`
