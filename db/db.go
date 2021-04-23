package db

import (
	"bytes"
	"errors"

	bolt "go.etcd.io/bbolt"
)

var (
	defaultBucket = []byte("default")
	replicaBucket = []byte("replication")
)

// Database is an open bolt database
type Database struct {
	db       *bolt.DB
	readOnly bool
}

// constructor
func NewDatabase(dbPath string, readOnly bool) (db *Database, closeFunc func() error, err error) {
	boltDb, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, nil, err
	}
	closeFunc = boltDb.Close

	db = &Database{boltDb, readOnly}
	if err := db.createDefaultBucket(); err != nil {
		closeFunc()
		return nil, nil, err
	}
	return
}

func (d *Database) createDefaultBucket() error {
	return d.db.Update(func(t *bolt.Tx) error {
		if _, err := t.CreateBucketIfNotExists(defaultBucket); err != nil {
			return err
		}

		if _, err := t.CreateBucketIfNotExists(replicaBucket); err != nil {
			return err
		}
		return nil
	})
}

// SetKey sets the key to the requested value or returns an error
func (d *Database) SetKey(key string, value []byte) error {
	if d.readOnly {
		return errors.New("read only mode")
	}
	return d.db.Update(func(t *bolt.Tx) error {
		if err := t.Bucket(defaultBucket).Put([]byte(key), value); err != nil {
			return err
		}
		return t.Bucket(replicaBucket).Put([]byte(key), value)
	})
}

// SetKey gets the value of the requested from a default database
func (d *Database) GetKey(key string) (res []byte, err error) {
	err = d.db.View(func(t *bolt.Tx) error {
		b := t.Bucket(defaultBucket)
		res = b.Get([]byte(key))
		return nil
	})
	return
}

func copyByteSlice(src []byte) []byte {
	if src == nil {
		return nil
	}
	dest := make([]byte, len(src))
	copy(dest, src)
	return dest
}

// GetNextForReplication returns the key and value for the keys that have
// changed and have not yet been applied to replicas
func (d *Database) GetNextForReplication() (key, value []byte, err error) {
	err = d.db.View(func(t *bolt.Tx) error {
		b := t.Bucket(replicaBucket)
		k, v := b.Cursor().First()
		key = copyByteSlice(k)
		value = copyByteSlice(v)
		return nil
	})

	if err != nil {
		key, value = nil, nil
	}
	return
}

// DeleteReplicationKey deletes the key from the replication queue
// if the value matches the contents or the key is already absent
func (d *Database) DeleteReplicationKey(key, value []byte) error {
	return d.db.Update(func(t *bolt.Tx) error {
		b := t.Bucket(replicaBucket)

		v := b.Get(key)
		if v == nil {
			return errors.New("key does not exist")
		}

		if !bytes.Equal(v, value) {
			return errors.New("value does not match")
		}
		return b.Delete(key)
	})
}

// DeleteExtraKeys delete the keys that do not belongs to this shard
func (d *Database) DeleteExtraKeys(isExtra func(string) bool) error {
	var keys []string
	err := d.db.View(func(t *bolt.Tx) error {
		b := t.Bucket(defaultBucket)
		return b.ForEach(func(k, v []byte) error {
			ks := string(k)
			if isExtra(ks) {
				keys = append(keys, ks)
			}
			return nil
		})
	})

	if err != nil {
		return err
	}

	return d.db.Update(func(t *bolt.Tx) error {
		b := t.Bucket(defaultBucket)

		for _, k := range keys {
			if err := b.Delete([]byte(k)); err != nil {
				return err
			}
		}
		return nil
	})
}
