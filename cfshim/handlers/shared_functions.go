package handlers

import (
	"encoding/json"
	"net/http"
)

func ReturnFormattedError(w http.ResponseWriter, status int, title string, detail string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(CFAPIErrors{
		Errors: []CFAPIError{
			{
				Title:  title,
				Detail: detail,
				Code:   code,
			},
		},
	})
}
