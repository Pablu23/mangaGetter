package database

import "database/sql"

type DbStatus int

const (
	New DbStatus = iota
	Loaded
	Updated
)

// Table TODO: This Could probably be a generic instead of interface / both
type Table[K comparable, T any] interface {
	Get(key K) (T, bool)
	Set(key K, new T)
	All() []T
	Save(db *sql.DB) error
	Load(db *sql.DB) error
}
