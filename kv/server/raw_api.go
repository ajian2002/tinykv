package server

import (
	"bytes"
	"context"
	"github.com/pingcap-incubator/tinykv/kv/storage"
	"github.com/pingcap-incubator/tinykv/proto/pkg/kvrpcpb"
)

// The functions below are Server's Raw API. (implements TinyKvServer).
// Some helper methods can be found in sever.go in the current directory

// RawGet return the corresponding Get response based on RawGetRequest's CF and Key fields
func (server *Server) RawGet(_ context.Context, req *kvrpcpb.RawGetRequest) (*kvrpcpb.RawGetResponse, error) {
	key := req.GetKey()
	cf := req.GetCf()
	cont := req.GetContext()
	res := new(kvrpcpb.RawGetResponse)
	res.NotFound = true
	res.Value = nil
	res.RegionError = nil //?
	res.Error = ""

	reader, err := server.storage.Reader(cont)
	if err == nil {
		//ok
		if reader != nil {
			defer reader.Close()
		}
		value, err := reader.GetCF(cf, key)
		if err != nil {
			res.Error = err.Error()
		}
		if value != nil {
			// founded
			res.Value = value
			res.NotFound = false
		}

	} else {
		//error
		res.Error = err.Error()
	}
	return res, nil
}

// RawPut puts the target data into storage and returns the corresponding response
func (server *Server) RawPut(_ context.Context, req *kvrpcpb.RawPutRequest) (*kvrpcpb.RawPutResponse, error) {
	// Your Code Here (1).
	// Hint: Consider using Storage.Modify to store data to be modified
	key := req.GetKey()
	cf := req.GetCf()
	newValue := req.GetValue()
	cont := req.GetContext()
	var mod []storage.Modify
	mod = append(mod, storage.Modify{
		Data: storage.Put{
			Key:   key,
			Value: newValue,
			Cf:    cf,
		},
	})
	res := new(kvrpcpb.RawPutResponse)
	res.RegionError = nil //?
	res.Error = ""
	err := server.storage.Write(cont, mod)
	if err != nil {
		//error
		res.Error = err.Error()
	}
	return res, nil
}

// RawDelete delete the target data from storage and returns the corresponding response
func (server *Server) RawDelete(_ context.Context, req *kvrpcpb.RawDeleteRequest) (*kvrpcpb.RawDeleteResponse, error) {
	// Your Code Here (1).
	// Hint: Consider using Storage.Modify to store data to be deleted

	key := req.GetKey()
	cf := req.GetCf()
	cont := req.GetContext()

	var mod []storage.Modify
	mod = append(mod, storage.Modify{
		Data: storage.Delete{
			Key: key,
			Cf:  cf,
		},
	})
	res := new(kvrpcpb.RawDeleteResponse)
	res.RegionError = nil //?
	res.Error = ""
	err := server.storage.Write(cont, mod)
	if err != nil {
		//error
		res.Error = err.Error()
	}
	return res, nil
}

// RawScan scan the data starting from the start key up to limit. and return the corresponding result
func (server *Server) RawScan(_ context.Context, req *kvrpcpb.RawScanRequest) (*kvrpcpb.RawScanResponse, error) {
	//return nil, nil
	// Your Code Here (1).
	// Hint: Consider using reader.IterCF
	key := req.GetStartKey()
	cf := req.GetCf()
	cont := req.GetContext()
	limit := req.GetLimit()

	res := new(kvrpcpb.RawScanResponse)
	res.RegionError = nil //?
	res.Error = ""
	res.Kvs = nil
	//	Kvs                  []*KvPair
	//type KvPair struct {
	//	Error                *KeyError `protobuf:"bytes,1,opt,name=error" json:"error,omitempty"`
	//	Key                  []byte    `protobuf:"bytes,2,opt,name=key,proto3" json:"key,omitempty"`
	//	Value                []byte    `protobuf:"bytes,3,opt,name=value,proto3" json:"value,omitempty"`
	//	XXX_NoUnkeyedLiteral struct{}  `json:"-"`
	//	XXX_unrecognized     []byte    `json:"-"`
	//	XXX_sizecache        int32     `json:"-"`
	//}
	reader, err := server.storage.Reader(cont)
	if reader == nil {
		res.Error = " Reader nil"
	} else {
		if err == nil {
			//ok
			defer reader.Close()
			dbIterator := reader.IterCF(cf)
			if dbIterator == nil {
				res.Error = "cf not engine_util.Cf*"
			} else {
				defer dbIterator.Close()
				dbIterator.Seek(key)
				if !bytes.Equal(dbIterator.Item().Key(), key) {
					res.Kvs = nil
					res.Error = "don't find key"
				} else {
					for i := uint32(0); i < limit && dbIterator.Valid(); i++ {
						Kvstru := new(kvrpcpb.KvPair)
						Kvstru.Key = dbIterator.Item().KeyCopy(Kvstru.Key)
						Kvstru.Value, err = dbIterator.Item().ValueCopy(Kvstru.Value)
						Kvstru.Error = nil
						res.Kvs = append(res.Kvs, Kvstru)
						dbIterator.Next()
					}
				}
			}
		} else {
			//error
			res.Error = err.Error()
		}
	}
	return res, nil
}
