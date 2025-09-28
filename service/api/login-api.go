package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"wasa-project/service/api/reqcontext"

	"github.com/gofrs/uuid"
	"github.com/julienschmidt/httprouter"
)

func (rt *_router) DoLogin(w http.ResponseWriter, r *http.Request, _ httprouter.Params, ctx reqcontext.RequestContext) {
	type loginRequest struct {
		Name string `json:"name"`
	}
	type loginResponse struct {
		Identifier string `json:"identifier"`
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid req", http.StatusBadRequest)
		return
	}
	if len(req.Name) < 3 || len(req.Name) > 16 {
		http.Error(w, "invalid name len", http.StatusBadRequest)
		return
	}

	u, err := rt.db.GetUserByUsername(req.Name)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err == nil && u != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(loginResponse{Identifier: u.ID})
		return
	}

	newID, err := uuid.NewV4()
	if err != nil {
		log.Println("failed to generate uuid:", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	err = rt.db.CreateUser(newID.String(), req.Name)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(loginResponse{Identifier: newID.String()})
}
