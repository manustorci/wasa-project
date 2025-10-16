/*
Package database is the middleware between the app database and the code. All data (de)serialization (save/load) from a
persistent database are handled here. Database specific logic should never escape this package.

To use this package you need to apply migrations to the database if needed/wanted, connect to it (using the database
data source name from config), and then initialize an instance of AppDatabase from the DB connection.

For example, this code adds a parameter in `webapi` executable for the database data source name (add it to the
main.WebAPIConfiguration structure):

	DB struct {
		Filename string `conf:""`
	}

This is an example on how to migrate the DB and connect to it:

	// Start Database
	logger.Println("initializing database support")
	db, err := sql.Open("sqlite3", "./foo.db")
	if err != nil {
		logger.WithError(err).Error("error opening SQLite DB")
		return fmt.Errorf("opening SQLite: %w", err)
	}
	defer func() {
		logger.Debug("database stopping")
		_ = db.Close()
	}()

Then you can initialize the AppDatabase and pass it to the api package.
*/
package database

import (
	"database/sql"
	"errors"
	"fmt"
)

// AppDatabase is the high level interface for the DB
type AppDatabase interface {

	//user
	CreateUser(id, username string) error
	GetUserByID(id string) (*User, error)
	GetUserByUsername(username string) (*User, error)
	SetUserPhoto(userID, photoPath string) error
	SetUsername(userID, newUsername string) error
	ListMessageComments(messageID int) ([]Comment, error)

	//group
	SendMessage(sender_id string, conversation_id int, text string) error
	CreateConversation(name string, isGroup bool, creatorID string) (int, error)
	AddUserToConversation(conversationID int, userID string) error
	GetConversationInfo(id int) (*ConversationInfo, error)
	IsUserInConversation(conversationID int, userID string) (bool, error)
	InsertMessage(conversation_id int, sender_id, text string) (int, error)
	FindDirectConversation(userA, userB string) (int, error)
	CreateDirectConversation(userA, userB string, name string) (int, error)
	GetMyConversations(userID string) ([]ConversationSummary, error)
	GetMessageByID(id int) (*Message, error)
	DeleteMessage(id int, authorID string) (bool, error)
	RemoveUserFromConversation(conversationID int, userID string) (bool, error)
	UpdateConversationName(id int, name string) error
	UpsertComment(messageID int, userID, comment string) (int, error)
	DeleteComment(messageID int, userID string) (bool, error)

	GetConversationParticipants(conversationID int) ([]string, error)
	ListConversationMessages(conversationID int) ([]Message, error)
	ListUsers(q string) ([]User, error)

	SetConversationPhoto(conversationID int, photoPath string) error

	Ping() error
}

type appdbimpl struct {
	c *sql.DB
}

// New returns a new instance of AppDatabase based on the SQLite connection `db`.
// `db` is required - an error will be returned if `db` is `nil`.
func New(db *sql.DB) (AppDatabase, error) {
	if db == nil {
		return nil, errors.New("database is required when building a AppDatabase")
	}

	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		return nil, fmt.Errorf("enabling foreign_keys: %w", err)
	}

	var tableName string

	//users
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='users';`).Scan(&tableName)
	if errors.Is(err, sql.ErrNoRows) {
		sqlStmt := `
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			photo TEXT
		);`
		_, err = db.Exec(sqlStmt)
		if err != nil {
			return nil, fmt.Errorf("error creating users table: %w", err)
		}
	}

	//conversations
	err = db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='conversations';`).Scan(&tableName)
	if errors.Is(err, sql.ErrNoRows) {
		sqlStmt := `
		CREATE TABLE conversations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT,
			is_group BOOLEAN NOT NULL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		);`
		_, err = db.Exec(sqlStmt)
		if err != nil {
			return nil, fmt.Errorf("error creating conversations table: %w", err)
		}
	}

	//photo
	var hasPhotoCol int
	err = db.QueryRow(`SELECT 1 FROM pragma_table_info('conversations') WHERE name='photo'`).Scan(&hasPhotoCol)
	if errors.Is(err, sql.ErrNoRows) {
		if _, err := db.Exec(`ALTER TABLE conversations ADD COLUMN photo TEXT`); err != nil {
			return nil, fmt.Errorf("adding conversations.photo: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("checking conversations.photo: %w", err)
	}

	//user_conversations
	err = db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='user_conversations';`).Scan(&tableName)
	if errors.Is(err, sql.ErrNoRows) {
		sqlStmt := `
		CREATE TABLE user_conversations (
			conversation_id INTEGER NOT NULL,
			user_id TEXT NOT NULL,
			PRIMARY KEY (conversation_id, user_id),
			FOREIGN KEY (conversation_id) REFERENCES conversations(id),
			FOREIGN KEY (user_id) REFERENCES users(id)

		);`
		_, err = db.Exec(sqlStmt)
		if err != nil {
			return nil, fmt.Errorf("error creating user_conversations table: %w", err)
		}
	}

	//messages
	err = db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='messages';`).Scan(&tableName)
	if errors.Is(err, sql.ErrNoRows) {
		sqlStmt := `
		CREATE TABLE messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id INTEGER NOT NULL,
			sender_id TEXT NOT NULL,
			text TEXT NOT NULL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (conversation_id) REFERENCES conversations(id),
    		FOREIGN KEY (sender_id) REFERENCES users(id)
		);`
		_, err = db.Exec(sqlStmt)
		if err != nil {
			return nil, fmt.Errorf("error creating messages table: %w", err)
		}
	}

	// comments
	err = db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='message_comments';`).Scan(&tableName)
	if errors.Is(err, sql.ErrNoRows) {
		sqlStmt := `
		CREATE TABLE message_comments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			message_id INTEGER NOT NULL,
			user_id TEXT NOT NULL,
			comment TEXT NOT NULL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (message_id, user_id),
			FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id)
		);`
		_, err = db.Exec(sqlStmt)
		if err != nil {
			return nil, fmt.Errorf("error creating message_comments table: %w", err)
		}
	}

	return &appdbimpl{
		c: db,
	}, nil
}

func (db *appdbimpl) Ping() error {
	return db.c.Ping()
}

func (db *appdbimpl) CreateUser(id, username string) error {
	_, err := db.c.Exec(
		"INSERT INTO users (id, username) VALUES (?, ?)",
		id,
		username,
	)
	if err != nil {
		return err
	}
	return nil
}

func (db *appdbimpl) SendMessage(sender_id string, conversation_id int, text string) error {
	_, err := db.c.Exec(`
		INSERT INTO messages (conversation_id, sender_id, text)
		VALUES (?,?,?)`,
		conversation_id, sender_id, text)
	return err
}

func (db *appdbimpl) CreateConversation(name string, isGroup bool, creatorID string) (int, error) {
	result, err := db.c.Exec(`
		INSERT INTO conversations (name, is_group)
		VALUES (?, ?)`,
		name, isGroup)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	_, err = db.c.Exec(`
        INSERT INTO user_conversations (conversation_id, user_id)
        VALUES (?, ?)`,
		id, creatorID)
	if err != nil {
		return int(id), fmt.Errorf("conversation created but failed to add creator: %w", err)
	}

	return int(id), nil
}

func (db *appdbimpl) AddUserToConversation(conversationID int, userID string) error {
	_, err := db.c.Exec(`
	INSERT INTO user_conversations (conversation_id, user_id)
	VALUES (?,?)`,
		conversationID, userID)
	return err
}

// esistenza is_group
type ConversationInfo struct {
	ID      int
	IsGroup bool
}

func (db *appdbimpl) GetConversationInfo(id int) (*ConversationInfo, error) {
	row := db.c.QueryRow(`SELECT id, is_group FROM conversations WHERE id = ?`, id)
	var info ConversationInfo
	if err := row.Scan(&info.ID, &info.IsGroup); err != nil {
		return nil, err // può essere sql.ErrNoRows
	}
	return &info, nil
}

func (db *appdbimpl) IsUserInConversation(conversationID int, userID string) (bool, error) {
	row := db.c.QueryRow(`
        SELECT 1
        FROM user_conversations
        WHERE conversation_id = ? AND user_id = ?
        LIMIT 1
    `, conversationID, userID)

	var one int
	if err := row.Scan(&one); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (db *appdbimpl) InsertMessage(conversation_id int, sender_id, text string) (int, error) {
	res, err := db.c.Exec(`
        INSERT INTO messages (conversation_id, sender_id, text)
        VALUES (?, ?, ?)`,
		conversation_id, sender_id, text)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func (db *appdbimpl) FindDirectConversation(userA, userB string) (int, error) {
	row := db.c.QueryRow(`
        SELECT c.id
        FROM conversations c
        JOIN user_conversations uc1 ON uc1.conversation_id = c.id AND uc1.user_id = ?
        JOIN user_conversations uc2 ON uc2.conversation_id = c.id AND uc2.user_id = ?
        WHERE c.is_group = 0
        LIMIT 1
    `, userA, userB)
	var id int
	if err := row.Scan(&id); err != nil {
		return 0, err // può essere sql.ErrNoRows
	}
	return id, nil
}

func (db *appdbimpl) CreateDirectConversation(userA, userB string, name string) (int, error) {
	id, err := db.CreateConversation(name, false, userA)
	if err != nil {
		return 0, err
	}
	if err := db.AddUserToConversation(id, userB); err != nil {
		return 0, err
	}
	return id, nil
}

func (db *appdbimpl) GetMyConversations(userID string) ([]ConversationSummary, error) {
	const q = `
		SELECT
		c.id,
		CASE
			WHEN c.is_group = 1 OR TRIM(IFNULL(c.name,'')) <> '' THEN c.name
			ELSE (
			SELECT u.username
			FROM user_conversations uc2
			JOIN users u ON u.id = uc2.user_id
			WHERE uc2.conversation_id = c.id AND uc2.user_id <> ?
			LIMIT 1
			)
		END AS display_name,
		c.is_group,
		(
			SELECT m.text
			FROM messages m
			WHERE m.conversation_id = c.id
			ORDER BY m.timestamp DESC
			LIMIT 1
		) AS last_text,
		(
			SELECT strftime('%Y-%m-%dT%H:%M:%SZ', m.timestamp)
			FROM messages m
			WHERE m.conversation_id = c.id
			ORDER BY m.timestamp DESC
			LIMIT 1
		) AS last_ts,
		CASE
			WHEN c.is_group = 1 AND TRIM(IFNULL(c.photo,'')) <> '' THEN c.photo
			ELSE (
			SELECT u.photo
			FROM user_conversations uc2
			JOIN users u ON u.id = uc2.user_id
			WHERE uc2.conversation_id = c.id AND uc2.user_id <> ?
			LIMIT 1
			)
		END AS photo_url
		FROM conversations c
		JOIN user_conversations uc ON uc.conversation_id = c.id
		WHERE uc.user_id = ?
		ORDER BY COALESCE(last_ts, strftime('%Y-%m-%dT%H:%M:%SZ', c.timestamp)) DESC
    `
	rows, err := db.c.Query(q, userID, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []ConversationSummary{}
	for rows.Next() {
		var it ConversationSummary
		if err := rows.Scan(&it.ID, &it.Name, &it.IsGroup, &it.LastText, &it.LastAtISO, &it.Photo); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (db *appdbimpl) GetMessageByID(id int) (*Message, error) {
	row := db.c.QueryRow(`
        SELECT id, conversation_id, sender_id, text, timestamp
        FROM messages
        WHERE id = ?`,
		id,
	)
	var m Message
	if err := row.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.Text, &m.Timestamp); err != nil {
		return nil, err
	}
	return &m, nil
}

func (db *appdbimpl) DeleteMessage(id int, authorID string) (bool, error) {
	res, err := db.c.Exec(`DELETE FROM messages WHERE id = ? AND sender_id = ?`, id, authorID)
	if err != nil {
		return false, err
	}
	aff, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return aff > 0, nil
}

func (db *appdbimpl) RemoveUserFromConversation(conversationID int, userID string) (bool, error) {
	res, err := db.c.Exec(
		`DELETE FROM user_conversations WHERE conversation_id = ? AND user_id = ?`,
		conversationID, userID,
	)
	if err != nil {
		return false, err
	}
	aff, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return aff > 0, nil
}

func (db *appdbimpl) UpdateConversationName(id int, name string) error {
	_, err := db.c.Exec(`UPDATE conversations SET name = ? WHERE id = ?`, name, id)
	return err
}

func (db *appdbimpl) UpsertComment(messageID int, userID, comment string) (int, error) {
	row := db.c.QueryRow(`
		INSERT INTO message_comments (message_id, user_id, comment)
		VALUES (?, ?, ?)
		ON CONFLICT(message_id, user_id) DO UPDATE
			SET comment = excluded.comment,
			    timestamp = CURRENT_TIMESTAMP
		RETURNING id
	`, messageID, userID, comment)

	var id int
	if err := row.Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (db *appdbimpl) DeleteComment(messageID int, userID string) (bool, error) {
	res, err := db.c.Exec(`DELETE FROM message_comments WHERE message_id = ? AND user_id = ?`,
		messageID, userID)
	if err != nil {
		return false, err
	}
	aff, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return aff > 0, nil
}

func (db *appdbimpl) SetUsername(userID, newUsername string) error {
	_, err := db.c.Exec(`UPDATE users SET username = ? WHERE id = ?`, newUsername, userID)
	return err
}

func (db *appdbimpl) SetUserPhoto(userID, photoPath string) error {
	_, err := db.c.Exec(`UPDATE users SET photo = ? WHERE id = ?`, photoPath, userID)
	return err
}

func (db *appdbimpl) SetConversationPhoto(conversationID int, photoPath string) error {
	_, err := db.c.Exec(`UPDATE conversations SET photo = ? WHERE id = ?`, photoPath, conversationID)
	return err
}

func (db *appdbimpl) GetConversationParticipants(conversationID int) ([]string, error) {
	rows, err := db.c.Query(`
        SELECT u.username
        FROM user_conversations uc
        JOIN users u ON u.id = uc.user_id
        WHERE uc.conversation_id = ?
        ORDER BY u.username`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (db *appdbimpl) ListConversationMessages(conversationID int) ([]Message, error) {
	rows, err := db.c.Query(`
        SELECT id, conversation_id, sender_id, text, timestamp
        FROM messages
        WHERE conversation_id = ?
        ORDER BY timestamp DESC`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.Text, &m.Timestamp); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (db *appdbimpl) ListUsers(q string) ([]User, error) {
	var rows *sql.Rows
	var err error
	if q == "" {
		rows, err = db.c.Query(`SELECT id, username, photo FROM users ORDER BY username`)
	} else {
		like := q + "%"
		rows, err = db.c.Query(`SELECT id, username, photo FROM users WHERE username LIKE ? ORDER BY username`, like)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.PhotoURL); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (db *appdbimpl) ListMessageComments(messageID int) ([]Comment, error) {
	rows, err := db.c.Query(`
        SELECT message_id, user_id, comment, timestamp
        FROM message_comments
        WHERE message_id = ?
        ORDER BY timestamp ASC
    `, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.MessageID, &c.UserID, &c.Comment, &c.Timestamp); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (db *appdbimpl) SearchUsersByName(query string) ([]User, error) {
	rows, err := db.c.Query(`
        SELECT id, username, photo
        FROM users
        WHERE username LIKE ?`, "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.PhotoURL); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}
