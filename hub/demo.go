package hub

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"time"
)

// demo exposes UNSECURE demo endpoints to test discovery and authorization mechanisms
// add a query parameter named "body" to define the content to return in the response's body
// add a query parameter named "jwt" set a "mercureAuthorization" cookie containing this token
// the Content-Type header will automatically be set according to the URL's extension
func demo(w http.ResponseWriter, r *http.Request) {
	// JSON-LD is the preferred format
	mime.AddExtensionType(".jsonld", "application/ld+json")

	url := r.URL.String()
	mimeType := mime.TypeByExtension(filepath.Ext(r.URL.Path))

	query := r.URL.Query()
	body := query.Get("body")
	jwt := query.Get("jwt")

	header := w.Header()
	// Several Link headers are set on purpose to allow testing advanced discovery mechanism
	header.Add("Link", "</subscribe>; rel=\"mercure\"")
	header.Add("Link", fmt.Sprintf("<%s>; rel=\"self\"", url))
	if mimeType != "" {
		header.Set("Content-Type", mimeType)
	}

	cookie := &http.Cookie{
		Name:     "mercureAuthorization",
		Path:     "/subscribe",
		Value:    jwt,
		HttpOnly: r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	}
	if jwt == "" {
		// Remove cookie if not provided, to be sure a previous one doesn't exist
		cookie.Expires = time.Unix(0, 0)
	}
	http.SetCookie(w, cookie)

	if body != "" {
		io.WriteString(w, body)
		return
	}

	doc := map[string]string{
		"@id":          url,
		"availability": "https://schema.org/InStock",
	}

	json, _ := json.Marshal(doc)
	w.Write(json)
}
