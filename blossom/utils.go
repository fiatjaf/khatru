package blossom

import "net/http"

func setCors(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization")
	w.Header().Set("Access-Control-Allow-Methods", "GET PUT DELETE")
}
