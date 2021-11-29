package engine_util

import (
	"os"

	"github.com/Connor1996/badger"
	"github.com/pingcap-incubator/tinykv/log"
)

// Engines keeps references to and data for the engines used by unistore.
// All engines are badger key/value databases.
// the Path fields are the filesystem path to where the data is stored.
type Engines struct {
	// Data, including data which is committed (i.e., committed across other nodes) and un-committed (i.e., only present
	// locally).
	//Package badger 实现了一个可嵌入的、简单且快速的键值数据库，它是用纯 Go 编写的。 它旨在同时为读取和写入提供高性能。 Badger 使用多版本并发控制 (MVCC)，并支持事务。 它同时运行事务，具有可序列化的快照隔离保证。
	//Badger 使用 LSM 树和值日志将键与值分开，从而减少写入放大和 LSM 树的大小。 这允许 LSM 树完全由 RAM 提供，而值则由 SSD 提供。
	Kv     *badger.DB
	KvPath string
	// Metadata used by Raft.
	Raft     *badger.DB
	RaftPath string
}

func NewEngines(kvEngine, raftEngine *badger.DB, kvPath, raftPath string) *Engines {
	return &Engines{
		Kv:       kvEngine,
		KvPath:   kvPath,
		Raft:     raftEngine,
		RaftPath: raftPath,
	}
}

func (en *Engines) WriteKV(wb *WriteBatch) error {
	return wb.WriteToDB(en.Kv)
}

func (en *Engines) WriteRaft(wb *WriteBatch) error {
	return wb.WriteToDB(en.Raft)
}

func (en *Engines) Close() error {
	if err := en.Kv.Close(); err != nil {
		return err
	}
	if err := en.Raft.Close(); err != nil {
		return err
	}
	return nil
}

func (en *Engines) Destroy() error {
	if err := en.Close(); err != nil {
		return err
	}
	if err := os.RemoveAll(en.KvPath); err != nil {
		return err
	}
	if err := os.RemoveAll(en.RaftPath); err != nil {
		return err
	}
	return nil
}

// CreateDB creates a new Badger DB on disk at subPath.
func CreateDB(path string, raft bool) *badger.DB {
	opts := badger.DefaultOptions
	if raft {
		// Do not need to write blob for raft engine because it will be deleted soon.
		opts.ValueThreshold = 0
	}
	opts.Dir = path
	opts.ValueDir = opts.Dir
	if err := os.MkdirAll(opts.Dir, os.ModePerm); err != nil {
		log.Fatal(err)
	}
	db, err := badger.Open(opts)
	if err != nil {
		log.Fatal(err)
	}
	return db
}
