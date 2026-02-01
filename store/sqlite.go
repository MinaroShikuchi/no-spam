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
		`CREATE TABLE IF NOT EXISTS subscriptions (
			topic TEXT,
			token TEXT,
			provider TEXT,
			PRIMARY KEY (topic, token)
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
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("error creating schema: %v", err)
		}
	}
	return nil
}

// Subscriptions
func (s *SQLiteStore) AddSubscription(topic, token, provider string) error {
	_, err := s.db.Exec(`INSERT INTO subscriptions (topic, token, provider) VALUES (?, ?, ?)`, topic, token, provider)
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

func (s *SQLiteStore) GetSubscriptionCount() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT count(*) FROM subscriptions`).Scan(&count)
	return count, err
}

// Save Message
func (s *SQLiteStore) SaveMessage(topic string, payload []byte) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO messages (topic, payload) VALUES (?, ?)`, topic, payload)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
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
