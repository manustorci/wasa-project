package api

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"wasa-project/service/api/reqcontext"

	"github.com/julienschmidt/httprouter"
)

func (rt *_router) SendDirectMessage(w http.ResponseWriter, r *http.Request, _ httprouter.Params, ctx reqcontext.RequestContext) {
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

	if _, err := rt.db.GetUserByID(req.ToUserID); err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Recipient not found", http.StatusNotFound)
			return
		}
		log.Printf("GetUserByID: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	convID, err := rt.db.FindDirectConversation(senderID, req.ToUserID)
	if err == sql.ErrNoRows {
		convID, err = rt.db.CreateDirectConversation(senderID, req.ToUserID, "")
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

	msgID, err := rt.db.InsertMessage(convID, senderID, req.Text)
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

func (rt *_router) ForwardMessage(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx reqcontext.RequestContext) {
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
	if _, err := rt.db.GetConversationInfo(dstConvID); err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		log.Printf("GetConversationInfo(dst): %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if ok, err := rt.db.IsUserInConversation(dstConvID, uid); err != nil {
		log.Printf("IsUserInConversation(dst): %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	} else if !ok {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// messaggio sorgente -> load + mship nella conversazione sorgente
	srcMsg, err := rt.db.GetMessageByID(msgID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		log.Printf("GetMessageByID: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if ok, err := rt.db.IsUserInConversation(srcMsg.ConversationID, uid); err != nil {
		log.Printf("IsUserInConversation(src): %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	} else if !ok {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// inserisco il messaggio nella destinazione
	if _, err := rt.db.InsertMessage(dstConvID, uid, srcMsg.Text); err != nil {
		log.Printf("InsertMessage(forward): %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "Forwarded"})
}

func (rt *_router) CommentMessage(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx reqcontext.RequestContext) {
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
	m, err := rt.db.GetMessageByID(msgID)
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
	ok, err := rt.db.IsUserInConversation(m.ConversationID, uid)
	if err != nil {
		log.Printf("IsUserInConversation: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	cmtID, err := rt.db.UpsertComment(msgID, uid, body.Comment)
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

func (rt *_router) UncommentMessage(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx reqcontext.RequestContext) {
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
	m, err := rt.db.GetMessageByID(msgID)
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
	ok, err := rt.db.IsUserInConversation(m.ConversationID, uid)
	if err != nil {
		log.Printf("IsUserInConversation: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	removed, err := rt.db.DeleteComment(msgID, uid)
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

func (rt *_router) DeleteMessage(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx reqcontext.RequestContext) {
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
	deleted, err := rt.db.DeleteMessage(msgID, uid)
	if err != nil {
		log.Printf("DeleteMessage: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !deleted {
		// on è stato cancellato nulla: distingui 404 vs 403
		m, err := rt.db.GetMessageByID(msgID)
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
