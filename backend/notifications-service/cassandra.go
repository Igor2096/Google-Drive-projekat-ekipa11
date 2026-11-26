package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gocql/gocql"
)

// Tabela je napravljena po upitu koji se izvrsava ("query-driven" dizajn,
// kako se radi u wide-column bazama): uvek se citaju notifikacije JEDNOG
// korisnika, najnovije prve.
//
//   - user_id je particioni kljuc: sve notifikacije korisnika su na istom cvoru
//   - created_at + notification_id su klasterujuce kolone: unutar particije
//     redovi su vec sortirani od najnovijeg, pa nije potreban ORDER BY
//   - user_id je TEXT jer je ID korisnika Mongo ObjectID (24 heks znaka) iz
//     Users servisa, a ne UUID
const createKeyspaceCQL = `CREATE KEYSPACE IF NOT EXISTS %s
  WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1}`

const createTableCQL = `CREATE TABLE IF NOT EXISTS %s.notifications_by_user (
  user_id         text,
  created_at      timestamp,
  notification_id timeuuid,
  message         text,
  is_read         boolean,
  PRIMARY KEY ((user_id), created_at, notification_id)
) WITH CLUSTERING ORDER BY (created_at DESC, notification_id DESC)`

// connect ceka da Cassandra proradi (posle "docker compose up" treba joj
// i do minut), pa pravi keyspace i tabelu ako ne postoje.
func connect(cfg config) (*gocql.Session, error) {
	cluster := gocql.NewCluster(cfg.CassandraHost)
	cluster.Port = cfg.CassandraPort
	cluster.Consistency = gocql.One
	cluster.Timeout = 10 * time.Second
	cluster.ConnectTimeout = 10 * time.Second

	var (
		bootstrap *gocql.Session
		err       error
	)
	for attempt := 1; attempt <= 40; attempt++ {
		bootstrap, err = cluster.CreateSession()
		if err == nil {
			break
		}
		log.Printf("Cassandra jos nije spremna (pokusaj %d/40): %v", attempt, err)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		return nil, err
	}

	// Sema se pravi kroz sesiju BEZ keyspace-a, jer keyspace mozda jos ne postoji.
	if err := bootstrap.Query(fmt.Sprintf(createKeyspaceCQL, cfg.Keyspace)).Exec(); err != nil {
		bootstrap.Close()
		return nil, fmt.Errorf("kreiranje keyspace-a: %w", err)
	}
	if err := bootstrap.Query(fmt.Sprintf(createTableCQL, cfg.Keyspace)).Exec(); err != nil {
		bootstrap.Close()
		return nil, fmt.Errorf("kreiranje tabele: %w", err)
	}
	bootstrap.Close()

	cluster.Keyspace = cfg.Keyspace
	return cluster.CreateSession()
}

type cassandraStore struct {
	session *gocql.Session
}

func (s *cassandraStore) ForUser(ctx context.Context, userID string) ([]Notification, error) {
	iter := s.session.Query(`
		SELECT notification_id, message, created_at, is_read
		FROM notifications_by_user
		WHERE user_id = ?`, userID).WithContext(ctx).Iter()

	result := []Notification{} // prazan niz, a ne null u JSON-u
	var (
		id        gocql.UUID
		message   string
		createdAt time.Time
		isRead    bool
	)
	for iter.Scan(&id, &message, &createdAt, &isRead) {
		result = append(result, Notification{
			UserID:         userID,
			NotificationID: id.String(),
			Message:        message,
			CreatedAt:      createdAt,
			IsRead:         isRead,
		})
	}
	if err := iter.Close(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *cassandraStore) Insert(ctx context.Context, n Notification) error {
	return s.session.Query(`
		INSERT INTO notifications_by_user (user_id, created_at, notification_id, message, is_read)
		VALUES (?, ?, now(), ?, ?)`,
		n.UserID, n.CreatedAt, n.Message, n.IsRead).WithContext(ctx).Exec()
}

// seedNotifications upisuje nekoliko probnih notifikacija za demonstraciju.
func seedNotifications(ctx context.Context, store *cassandraStore, userID string) error {
	if !validUserID(userID) {
		return fmt.Errorf("%q nije ispravan ID korisnika (ocekuje se 24 heks znaka)", userID)
	}
	now := time.Now().UTC()
	demo := []Notification{
		{Message: "Dobrodosli na Disk! Vas nalog je spreman.", CreatedAt: now.Add(-48 * time.Hour), IsRead: true},
		{Message: "Korisnik Ana Anic je podelila sa vama folder \"Projekat\".", CreatedAt: now.Add(-3 * time.Hour)},
		{Message: "Otpremljena je nova verzija datoteke \"specifikacija.pdf\".", CreatedAt: now.Add(-10 * time.Minute)},
	}
	for _, n := range demo {
		n.UserID = userID
		if err := store.Insert(ctx, n); err != nil {
			return err
		}
	}
	return nil
}
