package database

import "time"

type User struct {
	ID       string
	Username string
	PhotoURL *string
}

type Message struct {
	ID             int       `json:"id"`
	ConversationID int       `json:"conversation_id"`
	SenderID       string    `json:"sender_id"`
	Text           string    `json:"text"`
	Timestamp      time.Time `json:"timestamp"`
}

type Comment struct {
	MessageID int
	UserID    string
	Comment   string
	Timestamp time.Time
}

type Conversation struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	IsGroup   bool      `json:"is_group"`
	Timestamp time.Time `json:"timestamp"`
	PhotoURL  *string
}

type UserConversation struct {
	ConversationID int    `json:"conversation_id"`
	UserID         string `json:"user_id"`
}

type ConversationSummary struct {
	ID        int
	Name      string
	Photo     *string
	IsGroup   bool
	LastText  *string
	LastAt    *time.Time
	LastAtISO *string //mostra ultima attività in lista
}

func (db *appdbimpl) GetUserByID(id string) (*User, error) {
	row := db.c.QueryRow("SELECT id, username, photo FROM users WHERE id = ?", id)
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.PhotoURL)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

//func CreateUser

func (db *appdbimpl) GetUserByUsername(username string) (*User, error) {
	row := db.c.QueryRow("SELECT id, username, photo FROM users WHERE username = ?", username)
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.PhotoURL)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
