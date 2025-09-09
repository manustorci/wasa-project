package session

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
	"time"
	"wasa-project/service/database"

	"github.com/gofrs/uuid"
	"github.com/julienschmidt/httprouter"
)

type Handler struct {
	db database.AppDatabase
}

func NewHandler(db database.AppDatabase) *Handler {
	return &Handler{db: db}
}

func (h *Handler) DoLogin(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
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

	u, err := h.db.GetUserByUsername(req.Name)
	if err != nil && err != sql.ErrNoRows {
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

	err = h.db.CreateUser(newID.String(), req.Name)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(loginResponse{Identifier: newID.String()})
}

func (h *Handler) GetUserByID(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	if authUserID(r) == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	id := params.ByName("id")

	user, err := h.db.GetUserByID(id)
	if err == sql.ErrNoRows {
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

func (h *Handler) SendMessage(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
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
	if _, err := h.db.GetConversationInfo(conversationID); err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		log.Printf("GetConversationInfo: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// 403 se il caller non è membro
	ok, err := h.db.IsUserInConversation(conversationID, senderID)
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
	msgID, err := h.db.InsertMessage(conversationID, senderID, req.Text)
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

func (h *Handler) CreateConversation(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
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
	id, err := h.db.CreateConversation(req.Name, req.IsGroup, creatorID)
	if err != nil {
		http.Error(w, "could not create convesation", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(createConversationResponse{ConversationID: id})

}

func (h *Handler) AddUserToConversation(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
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
	info, err := h.db.GetConversationInfo(conversationID)
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
	if ok, _ := h.db.IsUserInConversation(conversationID, authUser); !ok {
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

	if err := h.db.AddUserToConversation(conversationID, req.UserID); err != nil {
		log.Printf("AddUserToConversation: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "Added"}) // 200
}

func (h *Handler) SendDirectMessage(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	type reqBody struct {
		ToUserID string `json:"toUserId"`
		Text     string `json:"text"`
	}
	type respBody struct {
		ConversationID int    `json:"conversationId"`
		MessageID      int    `json:"messageId"`
		Status         string `json:"status"`
	}

	senderID := authUserID(r)
	if strings.TrimSpace(senderID) == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	var req reqBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.ToUserID) == "" || strings.TrimSpace(req.Text) == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	if req.ToUserID == senderID {
		http.Error(w, "Bad request: cannot message yourself", http.StatusBadRequest)
		return
	}

	if _, err := h.db.GetUserByID(req.ToUserID); err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Recipient not found", http.StatusNotFound)
			return
		}
		log.Printf("GetUserByID: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	convID, err := h.db.FindDirectConversation(senderID, req.ToUserID)
	if err == sql.ErrNoRows {
		convID, err = h.db.CreateDirectConversation(senderID, req.ToUserID, "")
		if err != nil {
			log.Printf("CreateDirectConversation: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	} else if err != nil {
		log.Printf("FindDirectConversation: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	msgID, err := h.db.InsertMessage(convID, senderID, req.Text)
	if err != nil {
		log.Printf("InsertMessage: %v", err)
		http.Error(w, "failed to send message", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(respBody{
		ConversationID: convID,
		MessageID:      msgID,
		Status:         "sent",
	})
}

func (h *Handler) GetMyConversations(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
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

	convs, err := h.db.GetMyConversations(uid)
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

func (h *Handler) GetConversation(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
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
	if _, err := h.db.GetConversationInfo(convID); err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		log.Printf("GetConversationInfo: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// 403 se non sei membro
	ok, err := h.db.IsUserInConversation(convID, uid)
	if err != nil {
		log.Printf("IsUserInConversation: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	participants, err := h.db.GetConversationParticipants(convID)
	if err != nil {
		log.Printf("GetConversationParticipants: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	msgs, err := h.db.ListConversationMessages(convID)
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
		if u, err := h.db.GetUserByID(m.SenderID); err == nil && u != nil {
			senderName = u.Username
		}

		// prendi i commenti
		dbComments, _ := h.db.ListMessageComments(m.ID)
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

func (h *Handler) ForwardMessage(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	type reqBody struct {
		ConversationID int `json:"conversationId"`
	}

	uid := authUserID(r)
	if uid == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	msgIDStr := params.ByName("id")
	msgID, err := strconv.Atoi(msgIDStr)
	if err != nil || msgID <= 0 {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	var rb reqBody
	if err := json.NewDecoder(r.Body).Decode(&rb); err != nil || rb.ConversationID <= 0 {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	dstConvID := rb.ConversationID
	if err != nil || dstConvID <= 0 {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// conversazione di destinazione -> esistenza + membership
	if _, err := h.db.GetConversationInfo(dstConvID); err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		log.Printf("GetConversationInfo(dst): %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if ok, err := h.db.IsUserInConversation(dstConvID, uid); err != nil {
		log.Printf("IsUserInConversation(dst): %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	} else if !ok {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// messaggio sorgente -> load + mship nella conversazione sorgente
	srcMsg, err := h.db.GetMessageByID(msgID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		log.Printf("GetMessageByID: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if ok, err := h.db.IsUserInConversation(srcMsg.ConversationID, uid); err != nil {
		log.Printf("IsUserInConversation(src): %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	} else if !ok {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// inserisco il messaggio nella destinazione
	if _, err := h.db.InsertMessage(dstConvID, uid, srcMsg.Text); err != nil {
		log.Printf("InsertMessage(forward): %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "Forwarded"})
}

func (h *Handler) DeleteMessage(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	uid := authUserID(r)
	if uid == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	msgIDStr := params.ByName("id")
	msgID, err := strconv.Atoi(msgIDStr)
	if err != nil || msgID <= 0 {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// tentativo di cancellazione: id + author
	deleted, err := h.db.DeleteMessage(msgID, uid)
	if err != nil {
		log.Printf("DeleteMessage: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !deleted {
		// on è stato cancellato nulla: distingui 404 vs 403
		m, err := h.db.GetMessageByID(msgID)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Not found", http.StatusNotFound)
				return
			}
			log.Printf("GetMessageByID: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		// esiste ma non sei l'autore
		if m.SenderID != uid {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		// in teoria qui non si arriva
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

func (h *Handler) LeaveGroup(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
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
	info, err := h.db.GetConversationInfo(groupID)
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
	ok, err := h.db.IsUserInConversation(groupID, uid)
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
	removed, err := h.db.RemoveUserFromConversation(groupID, uid)
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

func (h *Handler) SetGroupName(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
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

	info, err := h.db.GetConversationInfo(groupID)
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

	ok, err := h.db.IsUserInConversation(groupID, uid)
	if err != nil {
		log.Printf("IsUserInConversation: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := h.db.UpdateConversationName(groupID, newName); err != nil {
		log.Printf("UpdateConversationName: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"name": newName})
}

func (h *Handler) CommentMessage(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	uid := authUserID(r)
	if uid == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	msgID, err := strconv.Atoi(params.ByName("id"))
	if err != nil || msgID <= 0 {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	type req struct {
		Comment string `json:"comment"`
	}

	var body req
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Comment) == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// 404 se il messaggio non esiste
	m, err := h.db.GetMessageByID(msgID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		log.Printf("GetMessageByID: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// 403 se il caller non è membro della conversazione del messaggio
	ok, err := h.db.IsUserInConversation(m.ConversationID, uid)
	if err != nil {
		log.Printf("IsUserInConversation: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	cmtID, err := h.db.UpsertComment(msgID, uid, body.Comment)
	if err != nil {
		log.Printf("UpsertComment: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		CommentID int    `json:"commentId"`
		Status    string `json:"status"`
	}{
		CommentID: cmtID,
		Status:    "ok",
	})
}

func (h *Handler) UncommentMessage(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	uid := authUserID(r)
	if uid == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	msgID, err := strconv.Atoi(params.ByName("id"))
	if err != nil || msgID <= 0 {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// 404 se il messaggio non esiste
	m, err := h.db.GetMessageByID(msgID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		log.Printf("GetMessageByID: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// 403 se il caller non è membro della conversazione del mex
	ok, err := h.db.IsUserInConversation(m.ConversationID, uid)
	if err != nil {
		log.Printf("IsUserInConversation: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	removed, err := h.db.DeleteComment(msgID, uid)
	if err != nil {
		log.Printf("DeleteComment: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !removed {
		// non avevo una reazione su quel messaggio -> 404
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "removed"})
}

func (h *Handler) SetMyUserName(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
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
	if _, err := h.db.GetUserByID(uid); err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		log.Printf("GetUserByID: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// unicità --> nome usato da un altro utente -> 400
	if u, err := h.db.GetUserByUsername(newName); err == nil && u != nil && u.ID != uid {
		http.Error(w, "Bad request: name already taken", http.StatusBadRequest)
		return
	} else if err != nil && err != sql.ErrNoRows {
		log.Printf("GetUserByUsername: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// update -> in caso di race, il vincolo UNIQUE farà fallire qui
	if err := h.db.SetUsername(uid, newName); err != nil {
		log.Printf("SetUsername: %v", err)
		http.Error(w, "Bad request: name already taken", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"name": newName})
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

func (h *Handler) SetMyPhoto(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	uid := authUserID(r)
	if uid == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	// 404 se l'utente non esiste
	if _, err := h.db.GetUserByID(uid); err != nil {
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
	if err := h.db.SetUserPhoto(uid, url); err != nil {
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

func (h *Handler) SetGroupPhoto(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
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

	info, err := h.db.GetConversationInfo(groupID)
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

	if ok, err := h.db.IsUserInConversation(groupID, uid); err != nil {
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
	if err := h.db.SetConversationPhoto(groupID, url); err != nil { // salva URL, non path FS
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

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	uid := authUserID(r)
	if uid == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	q := r.URL.Query().Get("q")
	users, err := h.db.ListUsers(q)
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
