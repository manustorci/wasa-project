package api

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"wasa-project/service/api/reqcontext"

	"github.com/julienschmidt/httprouter"
)

func (rt *_router) SendMessage(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx reqcontext.RequestContext) {
	type messageRequest struct {
		Text string `json:"text"`
	}

	type messageResponse struct {
		MessageID int    `json:"messageId"`
		Status    string `json:"status"`
	}

	senderID := authUserID(r)
	if senderID == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	conversationID, err := strconv.Atoi(params.ByName("id"))
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// 404 se la conversazione non esiste
	if _, err := rt.db.GetConversationInfo(conversationID); err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		log.Printf("GetConversationInfo: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// 403 se il caller non è membro
	ok, err := rt.db.IsUserInConversation(conversationID, senderID)
	if err != nil {
		log.Printf("IsUserInConversation: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var req messageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Text) == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Inserisci e ottieni l'ID del messaggio
	msgID, err := rt.db.InsertMessage(conversationID, senderID, req.Text)
	if err != nil {
		log.Printf("InsertMessage: %v", err)
		http.Error(w, "failed to send message", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(messageResponse{
		MessageID: msgID,
		Status:    "sent",
	})
}

func (rt *_router) CreateConversation(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx reqcontext.RequestContext) {
	type createConversationRequest struct {
		Name    string `json:"name"`
		IsGroup bool   `json:"isGroup"` //posso togliere
	}

	type createConversationResponse struct {
		ConversationID int `json:"conversationId"`
	}

	var req createConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	creatorID := authUserID(r)
	if creatorID == "" {
		http.Error(w, "missing authorization header", http.StatusUnauthorized)
		return
	}
	id, err := rt.db.CreateConversation(req.Name, req.IsGroup, creatorID)
	if err != nil {
		http.Error(w, "could not create convesation", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(createConversationResponse{ConversationID: id})

}

func (rt *_router) GetConversation(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx reqcontext.RequestContext) {
	uid := authUserID(r)
	if uid == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	convID, err := strconv.Atoi(params.ByName("id"))
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// 404 se non esiste
	if _, err := rt.db.GetConversationInfo(convID); err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		log.Printf("GetConversationInfo: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// 403 se non sei membro
	ok, err := rt.db.IsUserInConversation(convID, uid)
	if err != nil {
		log.Printf("IsUserInConversation: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	participants, err := rt.db.GetConversationParticipants(convID)
	if err != nil {
		log.Printf("GetConversationParticipants: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	msgs, err := rt.db.ListConversationMessages(convID)
	if err != nil {
		log.Printf("ListConversationMessages: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	type commentView struct {
		UserID  string `json:"userId"`
		Comment string `json:"comment"`
	}

	type msgView struct {
		ID        int           `json:"id"`
		Sender    string        `json:"sender"`
		Text      string        `json:"text"`
		Timestamp time.Time     `json:"timestamp"`
		Comments  []commentView `json:"comments"`
	}

	// mappa []database.Message ---> []msgView con username
	outMsgs := make([]msgView, 0, len(msgs))
	for _, m := range msgs {
		senderName := m.SenderID
		if u, err := rt.db.GetUserByID(m.SenderID); err == nil && u != nil {
			senderName = u.Username
		}

		// prendi i commenti
		dbComments, _ := rt.db.ListMessageComments(m.ID)
		cv := make([]commentView, 0, len(dbComments))
		for _, c := range dbComments {
			cv = append(cv, commentView{
				UserID:  c.UserID,
				Comment: c.Comment,
			})
		}

		outMsgs = append(outMsgs, msgView{
			ID:        m.ID,
			Sender:    senderName,
			Text:      m.Text,
			Timestamp: m.Timestamp,
			Comments:  cv,
		})
	}
	resp := struct {
		ID           int       `json:"id"`
		Participants []string  `json:"participants"`
		Messages     []msgView `json:"messages"`
	}{
		ID:           convID,
		Participants: participants,
		Messages:     outMsgs,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)

}
