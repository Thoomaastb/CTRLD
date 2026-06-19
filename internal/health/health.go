package health

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Thoomaastb/CTRLD/pkg/version"
)

// Response ist die Antwortstruktur des Health-Endpoints.
type Response struct {
	Status    string    `json:"status"`
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
}

// Handler gibt den Health-Status der Anwendung zurück.
// GET /api/v1/health → 200 OK, kein Auth erforderlich.
func Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)

	resp := Response{
		Status:    "ok",
		Version:   version.Version,
		Timestamp: time.Now().UTC(),
	}

	// Fehler beim Encoding können nach WriteHeader nicht mehr als HTTP-Status
	// zurückgegeben werden — wir loggen ihn nur (über den Server-Logger).
	_ = json.NewEncoder(w).Encode(resp)
}
