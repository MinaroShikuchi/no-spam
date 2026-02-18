package store

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	s := &SQLiteStore{db: db}
	if err := s.initSchema(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *SQLiteStore) initSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS topics (
			name TEXT PRIMARY KEY
		);`,
		`CREATE TABLE IF NOT EXISTS subscriptions (
			topic TEXT,
			token TEXT,
			provider TEXT,
			username TEXT,
			PRIMARY KEY (topic, token),
			FOREIGN KEY(topic) REFERENCES topics(name)
		);`,
		`CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			topic TEXT,
			payload BLOB,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS queue (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			message_id INTEGER,
			token TEXT,
			status TEXT DEFAULT 'pending',
			FOREIGN KEY(message_id) REFERENCES messages(id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_queue_token_status ON queue(token, status);`,
		`CREATE TABLE IF NOT EXISTS users (
			username TEXT PRIMARY KEY,
			password_hash TEXT,
			role TEXT
		);`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("error creating schema: %v", err)
		}
	}
	// Attempt to add username column if it doesn't exist (migration for dev)
	s.db.Exec(`ALTER TABLE subscriptions ADD COLUMN username TEXT;`)
	return nil
}

// Topics
func (s *SQLiteStore) CreateTopic(name string) error {
	_, err := s.db.Exec(`INSERT INTO topics (name) VALUES (?)`, name)
	return err
}

func (s *SQLiteStore) TopicExists(name string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM topics WHERE name = ?)`, name).Scan(&exists)
	return exists, err
}

func (s *SQLiteStore) ListTopics() ([]string, error) {
	rows, err := s.db.Query(`SELECT name FROM topics`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var topics []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		topics = append(topics, name)
	}
	return topics, nil
}

func (s *SQLiteStore) DeleteTopic(name string) error {
	// Check if topic has messages
	var msgCount int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE topic = ?`, name).Scan(&msgCount)
	if err != nil {
		return fmt.Errorf("failed to check messages: %w", err)
	}
	if msgCount > 0 {
		return fmt.Errorf("cannot delete topic: has %d messages", msgCount)
	}

	// Check if topic has subscribers
	var subCount int
	err = s.db.QueryRow(`SELECT COUNT(*) FROM subscriptions WHERE topic = ?`, name).Scan(&subCount)
	if err != nil {
		return fmt.Errorf("failed to check subscribers: %w", err)
	}
	if subCount > 0 {
		return fmt.Errorf("cannot delete topic: has %d subscribers", subCount)
	}

	// Delete topic
	_, err = s.db.Exec(`DELETE FROM topics WHERE name = ?`, name)
	return err
}

// Subscriptions
func (s *SQLiteStore) AddSubscription(topic, token, provider, username string) error {
	_, err := s.db.Exec(`INSERT INTO subscriptions (topic, token, provider, username) VALUES (?, ?, ?, ?)`, topic, token, provider, username)
	if err != nil {
		// Check for constraint violation? For now, standard error is fine, caller can infer.
		// Or return specific error "already subscribed"
		return fmt.Errorf("failed to subscribe: %w", err)
	}
	return nil
}

func (s *SQLiteStore) RemoveSubscription(topic, token string) error {
	_, err := s.db.Exec(`DELETE FROM subscriptions WHERE topic = ? AND token = ?`, topic, token)
	return err
}

func (s *SQLiteStore) ClearTopicSubscribers(topic string) error {
	_, err := s.db.Exec(`DELETE FROM subscriptions WHERE topic = ?`, topic)
	return err
}

func (s *SQLiteStore) GetSubscribers(topic string) ([]Subscriber, error) {
	rows, err := s.db.Query(`SELECT topic, token, provider FROM subscriptions WHERE topic = ?`, topic)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []Subscriber
	for rows.Next() {
		var sub Subscriber
		if err := rows.Scan(&sub.Topic, &sub.Token, &sub.Provider); err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	return subs, nil
}

func (s *SQLiteStore) GetSubscriptionsByUser(username string) ([]Subscriber, error) {
	rows, err := s.db.Query(`SELECT topic, token, provider FROM subscriptions WHERE username = ?`, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []Subscriber
	for rows.Next() {
		var sub Subscriber
		if err := rows.Scan(&sub.Topic, &sub.Token, &sub.Provider); err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	return subs, nil
}

func (s *SQLiteStore) GetSubscriptionsByToken(token string) ([]Subscriber, error) {
	rows, err := s.db.Query(`SELECT topic, token, provider FROM subscriptions WHERE token = ?`, token)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []Subscriber
	for rows.Next() {
		var sub Subscriber
		if err := rows.Scan(&sub.Topic, &sub.Token, &sub.Provider); err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	return subs, nil
}

func (s *SQLiteStore) GetSubscriptionCount() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT count(*) FROM subscriptions`).Scan(&count)
	return count, err
}

// Users
func (s *SQLiteStore) CreateUser(username, passwordHash, role string) error {
	_, err := s.db.Exec(`INSERT INTO users (username, password_hash, role) VALUES (?, ?, ?)`, username, passwordHash, role)
	return err
}

func (s *SQLiteStore) GetUser(username string) (*User, error) {
	var u User
	err := s.db.QueryRow(`SELECT username, password_hash, role FROM users WHERE username = ?`, username).Scan(&u.Username, &u.PasswordHash, &u.Role)
	if err == sql.ErrNoRows {
		return nil, nil // Not found
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *SQLiteStore) HasAdminUser() (bool, error) {
	var exists bool
	err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE role = 'admin')`).Scan(&exists)
	return exists, err
}

func (s *SQLiteStore) UpdateUserRole(username, role string) error {
	_, err := s.db.Exec(`UPDATE users SET role = ? WHERE username = ?`, role, username)
	return err
}

// Save Message
func (s *SQLiteStore) SaveMessage(topic string, payload []byte) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO messages (topic, payload) VALUES (?, ?)`, topic, payload)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *SQLiteStore) GetRecentMessages(topic string, limit int) ([]Message, error) {
	// Fetch newest first to respect limit
	query := `SELECT id, topic, payload, created_at FROM messages WHERE topic = ? ORDER BY created_at DESC LIMIT ?`
	rows, err := s.db.Query(query, topic, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var msg Message
		if err := rows.Scan(&msg.ID, &msg.Topic, &msg.Payload, &msg.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, msg)
	}

	// Reverse to return Oldest -> Newest (Chronological)
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}

	return msgs, nil
}

func (s *SQLiteStore) ClearTopicMessages(topic string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete from queue first (constraint)
	_, err = tx.Exec(`DELETE FROM queue WHERE message_id IN (SELECT id FROM messages WHERE topic = ?)`, topic)
	if err != nil {
		return err
	}

	// Delete messages
	_, err = tx.Exec(`DELETE FROM messages WHERE topic = ?`, topic)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// Queue
func (s *SQLiteStore) EnqueueMessage(messageID int64, token string) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO queue (message_id, token, status) VALUES (?, ?, 'pending')`, messageID, token)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *SQLiteStore) GetPendingMessages(token string) ([]QueueItem, error) {
	query := `
		SELECT q.id, q.message_id, q.token, q.status, m.payload 
		FROM queue q
		JOIN messages m ON q.message_id = m.id
		WHERE q.token = ? AND q.status = 'pending'
		ORDER BY m.created_at ASC
	`
	rows, err := s.db.Query(query, token)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []QueueItem
	for rows.Next() {
		var item QueueItem
		if err := rows.Scan(&item.ID, &item.MessageID, &item.Token, &item.Status, &item.Payload); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *SQLiteStore) GetAllPendingMessages() ([]QueueItem, error) {
	query := `
		SELECT q.id, q.message_id, q.token, sub.provider, q.status, m.payload 
		FROM queue q
		JOIN messages m ON q.message_id = m.id
		JOIN subscriptions sub ON q.token = sub.token
		WHERE q.status = 'pending'
		ORDER BY m.created_at ASC
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []QueueItem
	for rows.Next() {
		var item QueueItem
		if err := rows.Scan(&item.ID, &item.MessageID, &item.Token, &item.Provider, &item.Status, &item.Payload); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *SQLiteStore) MarkDelivered(queueID int64) error {
	_, err := s.db.Exec(`UPDATE queue SET status = 'delivered' WHERE id = ?`, queueID)
	return err
}

// Stats
func (s *SQLiteStore) GetTotalMessagesSent() (int64, error) {
	var count int64
	err := s.db.QueryRow(`SELECT count(*) FROM messages`).Scan(&count)
	return count, err
}
