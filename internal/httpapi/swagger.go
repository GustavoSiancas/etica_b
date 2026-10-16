package httpapi

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.json
var openAPISpec []byte

const swaggerPage = `<!doctype html>
<html lang="es">
<head>
  <meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
  <title>Swagger UI · Ética CAL</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui.css">
  <style>body{margin:0;background:#fafafa}</style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"></script>
  <script src="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-standalone-preset.js"></script>
  <script>window.onload=()=>SwaggerUIBundle({url:'/openapi.json',dom_id:'#swagger-ui',deepLinking:true,presets:[SwaggerUIBundle.presets.apis,SwaggerUIStandalonePreset],layout:'StandaloneLayout'});</script>
</body>
</html>`

func registerSwagger(mux *http.ServeMux){
	mux.HandleFunc("GET /openapi.json",func(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","application/json; charset=utf-8");w.Write(openAPISpec)})
	mux.HandleFunc("GET /swagger",func(w http.ResponseWriter,r *http.Request){http.Redirect(w,r,"/swagger/",http.StatusPermanentRedirect)})
	mux.HandleFunc("GET /swagger/",func(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","text/html; charset=utf-8");w.Write([]byte(swaggerPage))})
}
