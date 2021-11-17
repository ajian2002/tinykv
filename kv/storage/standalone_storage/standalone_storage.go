package standalone_storage

import (
	"github.com/Connor1996/badger"
	"github.com/pingcap-incubator/tinykv/kv/config"
	"github.com/pingcap-incubator/tinykv/kv/storage"
	"github.com/pingcap-incubator/tinykv/proto/pkg/kvrpcpb"
	"path"
)

// StandAloneStorage is an implementation of `Storage` for a single-node TinyKV instance. It does not
// communicate with other nodes and all data is stored locally.
type StandAloneStorage struct {
	// Your Data Here (1).
	conf    *config.Config
	engines *engine_util.Engines
}

// dbReader is a StorageReader which reads from a db.
type dbReader struct {
	txn     *badger.Txn
	engines *engine_util.Engines
}

func NewStandAloneStorage(conf *config.Config) *StandAloneStorage {
	s := new(StandAloneStorage)
	s.conf = conf
	return s
}

func (s *StandAloneStorage) Start() error {
	kvpath := path.Join(s.conf.DBPath, "kv")
	raftpath := path.Join(s.conf.DBPath, "raft")
	//kvpath := s.conf.DBPath
	db := engine_util.CreateDB(kvpath, false)
	radb := engine_util.CreateDB(raftpath, true)
	s.engines = engine_util.NewEngines(db, radb, kvpath, raftpath)
	//fmt.Println(s.engines)
	return nil
}

func (s *StandAloneStorage) Stop() error {
	return s.engines.Close()
}

func (s *StandAloneStorage) Reader(ctx *kvrpcpb.Context) (storage.StorageReader, error) {
	//该DB.View()和DB.Update()方法是围绕着包装 DB.NewTransaction()和Txn.Commit()方法（或Txn.Discard()在只读交易的情况下）。这些辅助方法将启动事务，执行函数，然后在返回错误时安全地丢弃您的事务。这是使用 Badger 交易的推荐方式。
	//
	//但是，有时您可能希望手动创建和提交事务。您可以DB.NewTransaction()直接使用该函数，它接受一个布尔参数来指定是否需要读写事务。对于读写事务，需要调用Txn.Commit() 以确保事务提交。对于只读事务，调用 Txn.Discard()就足够了。Txn.Commit()也会在Txn.Discard() 内部调用以清理事务，因此Txn.Commit()对于读写事务，只需调用就足够了。但是，如果您的代码Txn.Commit()由于某种原因没有调用 （例如它过早返回并出现错误），那么请确保您Txn.Discard()在defer块中调用。请参考下面的代码。
	//https://pkg.go.dev/github.com/Connor1996/badger@v1.5.1-0.20210202034640-5ff470f827f8#section-readme
	reader := new(dbReader)
	reader.txn = s.engines.Kv.NewTransaction(false)
	reader.engines = s.engines
	return reader, nil
}

func (s *StandAloneStorage) Write(ctx *kvrpcpb.Context, batch []storage.Modify) error {
	// Your Code Here (1).
	//var br engine_util.WriteBatch
	br := new(engine_util.WriteBatch)
	for _, b := range batch {
		switch b.Data.(type) {
		case storage.Put:
			//err := engine_util.PutCF(s.engines.Kv, b.Cf(), b.Key(), b.Value())
			br.SetCF(b.Cf(), b.Key(), b.Value())
		case storage.Delete:
			br.DeleteCF(b.Cf(), b.Key())
		}
	}
	return br.WriteToDB(s.engines.Kv)
}
func (s *dbReader) GetCF(cf string, key []byte) ([]byte, error) {
	result, err := engine_util.GetCFFromTxn(s.txn, cf, key)
	if err == badger.ErrKeyNotFound {
		return nil, nil
	}
	return result, err
	//return engine_util.GetCF(s.engines.Kv, cf, key)
}

func (s *dbReader) IterCF(cf string) engine_util.DBIterator {
	return engine_util.NewCFIterator(cf, s.txn)
}
func (s *dbReader) Close() {
	s.txn.Discard()
}
