package engine_util

/*
An engine is a low-level system for storing key/value pairs locally (without distribution or any transaction support,
etc.). This package contains code for interacting with such engines.

CF means 'column family'. A good description of column families is given in https://github.com/facebook/rocksdb/wiki/Column-Families
(specifically for RocksDB, but the general concepts are universal). In short, a column family is a key namespace.
Multiple column families are usually implemented as almost separate databases. Importantly each column family can be
configured separately. Writes can be made atomic across column families, which cannot be done for separate databases.

engine_util includes the following packages:

* engines: a data structure for keeping engines required by unistore.  引擎：用于保持 unistore 所需引擎的数据结构。
* write_batch: code to batch writes into a single, atomic 'transaction'.批量写入单个原子“事务”的代码。
* cf_iterator: code to iterate over a whole column family in badger. cf_iterator：在獾中遍历整个列族的代码。
*/
