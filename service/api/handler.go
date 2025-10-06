package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

/*
	type Handler struct {
		db database.AppDatabase
	}

	func NewHandler(db database.AppDatabase) *Handler {
		return &Handler{db: db}
	}
*/

func (rt *_router) ListUsers(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	uid := authUserID(r)
	if uid == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	q := r.URL.Query().Get("q")
	users, err := rt.db.ListUsers(q)
	if err != nil {
		log.Printf("ListUsers: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	type userView struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	resp := make([]userView, 0, len(users))
	for _, u := range users {
		resp = append(resp, userView{ID: u.ID, Name: u.Username})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
