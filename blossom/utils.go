package blossom

import (
	"mime"
	"net/http"
)

func blossomError(w http.ResponseWriter, msg string, code int) {
	w.Header().Add("X-Reason", msg)
	w.WriteHeader(code)
}

func blossomRedirect(w http.ResponseWriter, redir string, code int) {
	w.Header().Set("Location", redir)
	switch code {
	case 300:
		w.WriteHeader(http.StatusMultipleChoices)
	case 301:
		w.WriteHeader(http.StatusMovedPermanently)
	case 302:
		w.WriteHeader(http.StatusFound)
	case 303:
		w.WriteHeader(http.StatusSeeOther)
	case 304:
		w.WriteHeader(http.StatusNotModified)
	case 305:
		w.WriteHeader(http.StatusUseProxy)
	case 307:
		w.WriteHeader(http.StatusTemporaryRedirect)
	case 308:
		w.WriteHeader(http.StatusPermanentRedirect)
	}
}

func getExtension(mimetype string) string {
	if mimetype == "" {
		return ""
	}

	switch mimetype {
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "video/mp4":
		return ".mp4"
	}

	exts, _ := mime.ExtensionsByType(mimetype)
	if len(exts) > 0 {
		return exts[0]
	}

	return ""
}
