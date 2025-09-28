package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"wasa-project/service/api/reqcontext"

	"github.com/julienschmidt/httprouter"
)

func (rt *_router) SetMyUserName(w http.ResponseWriter, r *http.Request, _ httprouter.Params, ctx reqcontext.RequestContext) {
	uid := authUserID(r)
	if uid == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	type reqBody struct {
		Name string `json:"name"`
	}
	var req reqBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	newName := strings.TrimSpace(req.Name)
	if len(newName) < 3 || len(newName) > 16 {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// 404 se il bearer non mappa a un utente
	if _, err := rt.db.GetUserByID(uid); err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		log.Printf("GetUserByID: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// unicità --> nome usato da un altro utente -> 400
	if u, err := rt.db.GetUserByUsername(newName); err == nil && u != nil && u.ID != uid {
		http.Error(w, "Bad request: name already taken", http.StatusBadRequest)
		return
	} else if err != nil && err != sql.ErrNoRows {
		log.Printf("GetUserByUsername: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// update -> in caso di race, il vincolo UNIQUE farà fallire qui
	if err := rt.db.SetUsername(uid, newName); err != nil {
		log.Printf("SetUsername: %v", err)
		http.Error(w, "Bad request: name already taken", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"name": newName})
}

func (rt *_router) SetMyPhoto(w http.ResponseWriter, r *http.Request, _ httprouter.Params, ctx reqcontext.RequestContext) {
	uid := authUserID(r)
	if uid == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	// 404 se l'utente non esiste
	if _, err := rt.db.GetUserByID(uid); err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		log.Printf("GetUserByID: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// parse multipart
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("photo")
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer file.Close()

	lr := &io.LimitedReader{R: file, N: maxUploadSize + 1}
	data, err := io.ReadAll(lr)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if int64(len(data)) > maxUploadSize {
		http.Error(w, "Bad request: file too large", http.StatusBadRequest)
		return
	}

	ext, ok := detectImageExt(data)
	if !ok {
		http.Error(w, "Bad request: unsupported image type", http.StatusBadRequest)
		return
	}

	if err := os.MkdirAll("uploads/users", 0o755); err != nil {
		log.Printf("MkdirAll: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	dstFS := filepath.Join("uploads", "users", uid+ext)
	if err := os.WriteFile(dstFS, data, 0o644); err != nil {
		log.Printf("WriteFile(%s): %v (orig: %s)", dstFS, err, header.Filename)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	url := "/uploads/users/" + uid + ext
	if err := rt.db.SetUserPhoto(uid, url); err != nil {
		log.Printf("SetUserPhoto: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "Photo uploaded",
		"url":     url,
	})
}

func (rt *_router) GetMyConversations(w http.ResponseWriter, r *http.Request, _ httprouter.Params, ctx reqcontext.RequestContext) {
	type item struct {
		ID              int     `json:"id"`
		Name            string  `json:"name"`
		IsGroup         bool    `json:"isGroup"`
		LastMessageText *string `json:"lastMessageText,omitempty"`
		LastMessageAt   *string `json:"lastMessageAt,omitempty"`
		PhotoURL        *string `json:"photoUrl,omitempty"`
	}

	uid := authUserID(r)
	if uid == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	convs, err := rt.db.GetMyConversations(uid)
	if err != nil {
		log.Printf("GetMyConversations: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	out := make([]item, 0, len(convs))
	for _, c := range convs {
		out = append(out, item{
			ID:              c.ID,
			Name:            c.Name,
			IsGroup:         c.IsGroup,
			LastMessageText: c.LastText,
			LastMessageAt:   c.LastAtISO,
			PhotoURL:        c.Photo,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (rt *_router) GetUserByID(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx reqcontext.RequestContext) {
	if authUserID(r) == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	id := params.ByName("id")

	user, err := rt.db.GetUserByID(id)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Println("GetUserByID error:", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	type userResponse struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	resp := userResponse{ID: user.ID, Name: user.Username}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func authUserID(r *http.Request) string {
	raw := strings.TrimSpace(r.Header.Get("Authorization"))
	if raw == "" {
		return ""
	}
	const p = "Bearer "
	if len(raw) >= len(p) && strings.HasPrefix(raw, p) {
		return strings.TrimSpace(raw[len(p):])
	}
	return raw
}
