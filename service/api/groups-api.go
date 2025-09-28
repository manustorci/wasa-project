package api

import (
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"wasa-project/service/api/reqcontext"

	"github.com/julienschmidt/httprouter"
)

func (rt *_router) AddUserToConversation(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx reqcontext.RequestContext) {
	authUser := authUserID(r)
	if authUser == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	conversationID, err := strconv.Atoi(params.ByName("id"))
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// 404 se non esiste
	info, err := rt.db.GetConversationInfo(conversationID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		log.Printf("GetConversationInfo: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// 400 se non è un gruppo
	if !info.IsGroup {
		http.Error(w, "Bad request: Not a group conversation", http.StatusBadRequest)
		return
	}

	// 403 se il caller non è membro del gruppo
	if ok, _ := rt.db.IsUserInConversation(conversationID, authUser); !ok {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	type addUserReq struct {
		UserID string `json:"userId"`
	}

	var req addUserReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.UserID) == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if err := rt.db.AddUserToConversation(conversationID, req.UserID); err != nil {
		log.Printf("AddUserToConversation: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "Added"}) // 200
}

func (rt *_router) SetGroupName(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx reqcontext.RequestContext) {
	uid := authUserID(r)
	if uid == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	groupID, err := strconv.Atoi(params.ByName("id"))
	if err != nil || groupID <= 0 {
		http.Error(w, "Bad request", http.StatusBadRequest)
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
	if newName == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	info, err := rt.db.GetConversationInfo(groupID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		log.Printf("GetConversationInfo: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !info.IsGroup {
		http.Error(w, "Bad request: Not a group conversation", http.StatusBadRequest)
		return
	}

	ok, err := rt.db.IsUserInConversation(groupID, uid)
	if err != nil {
		log.Printf("IsUserInConversation: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := rt.db.UpdateConversationName(groupID, newName); err != nil {
		log.Printf("UpdateConversationName: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"name": newName})
}

func (rt *_router) SetGroupPhoto(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx reqcontext.RequestContext) {
	uid := authUserID(r)
	if uid == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	groupID, err := strconv.Atoi(params.ByName("id"))
	if err != nil || groupID <= 0 {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	info, err := rt.db.GetConversationInfo(groupID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		log.Printf("GetConversationInfo: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !info.IsGroup {
		http.Error(w, "Bad request: Not a group conversation", http.StatusBadRequest)
		return
	}

	if ok, err := rt.db.IsUserInConversation(groupID, uid); err != nil {
		log.Printf("IsUserInConversation: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	} else if !ok {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

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

	if err := os.MkdirAll("uploads/groups", 0o755); err != nil { // <- relativo, non "/uploads/..."
		log.Printf("MkdirAll: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	dstFS := filepath.Join("uploads", "groups", strconv.Itoa(groupID)+ext)
	if err := os.WriteFile(dstFS, data, 0o644); err != nil {
		log.Printf("WriteFile(%s): %v (orig: %s)", dstFS, err, header.Filename)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	url := "/uploads/groups/" + strconv.Itoa(groupID) + ext
	if err := rt.db.SetConversationPhoto(groupID, url); err != nil { // salva URL, non path FS
		log.Printf("SetConversationPhoto: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "Photo uploaded",
		"url":     url,
	})
}

func (rt *_router) LeaveGroup(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx reqcontext.RequestContext) {
	uid := authUserID(r)
	if uid == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	groupID, err := strconv.Atoi(params.ByName("id"))
	if err != nil || groupID <= 0 {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// 404 se la conversazione/gruppo non esiste
	info, err := rt.db.GetConversationInfo(groupID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		log.Printf("GetConversationInfo: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// 400 se non è un gruppo
	if !info.IsGroup {
		http.Error(w, "Bad request: Not a group conversation", http.StatusBadRequest)
		return
	}

	// 404 se non sei già membro
	ok, err := rt.db.IsUserInConversation(groupID, uid)
	if err != nil {
		log.Printf("IsUserInConversation: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// rimuovo la membership
	removed, err := rt.db.RemoveUserFromConversation(groupID, uid)
	if err != nil {
		log.Printf("RemoveUserFromConversation: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !removed {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "Left"})
}

const maxUploadSize = int64(10 << 20) // 10 MB

func detectImageExt(data []byte) (string, bool) {
	ctype := http.DetectContentType(data[:min(512, len(data))])
	switch ctype {
	case "image/jpeg":
		return ".jpg", true
	case "image/png":
		return ".png", true
	case "image/webp":
		return ".webp", true
	case "image/gif":
		return ".gif", true
	default:
		return "", false
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
