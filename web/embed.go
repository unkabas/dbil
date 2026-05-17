// Package web embeds the compiled SPA bundle so the dbil binary can serve it
// without any external static files. The frontend lives in web/ (Vite +
// React + TypeScript); `npm run build` populates web/dist with the artifacts
// embedded here.
package web

import "embed"

// DistFS holds the built frontend. The embed directive matches everything
// inside web/dist including dotfiles (e.g. .gitkeep) so the directive never
// fails when the bundle has not been built yet.
//
//go:embed all:dist
var DistFS embed.FS
