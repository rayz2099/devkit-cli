package dsn

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func queryMongo(ctx context.Context, raw, stmt string, connect, exec time.Duration) (QueryOut, error) {
	var doc bson.D
	if err := bson.UnmarshalExtJSON([]byte(stmt), true, &doc); err != nil {
		return QueryOut{}, exitf(3, "%s", err.Error())
	}
	opts := options.Client().ApplyURI(raw).
		SetServerSelectionTimeout(connect).
		SetConnectTimeout(connect).
		SetSocketTimeout(exec)
	dialCtx, cancel := context.WithTimeout(ctx, connect)
	client, err := mongo.Connect(dialCtx, opts)
	cancel()
	if err != nil {
		return QueryOut{}, exitf(3, "%s", err.Error())
	}
	defer client.Disconnect(context.Background())
	dbName, err := mongoDBName(raw)
	if err != nil {
		return QueryOut{}, err
	}
	// 空 path 跟驱动默认库走，不能发明一个业务库名。
	name := dbName
	if name == "" {
		name = "test"
	}
	qctx, qcancel := context.WithTimeout(ctx, exec)
	defer qcancel()
	var out bson.D
	if err := client.Database(name).RunCommand(qctx, doc).Decode(&out); err != nil {
		return QueryOut{}, exitf(3, "%s", err.Error())
	}
	return mongoOut(out), nil
}

func mongoOut(out bson.D) QueryOut {
	plain := bsonDToAny(out)
	if obj, ok := plain.(map[string]any); ok {
		if cur, ok := obj["cursor"].(map[string]any); ok {
			if batch, ok := cur["firstBatch"].([]any); ok {
				return docsOut(batch)
			}
		}
		return docsOut([]any{obj})
	}
	return docsOut([]any{plain})
}

func bsonDToAny(d bson.D) any {
	if d == nil {
		return nil
	}
	obj := make(map[string]any, len(d))
	for _, elem := range d {
		obj[elem.Key] = bsonToAny(elem.Value)
	}
	return obj
}

func bsonToAny(v any) any {
	switch t := v.(type) {
	case bson.D:
		return bsonDToAny(t)
	case bson.A:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = bsonToAny(item)
		}
		return out
	case bson.M:
		obj := make(map[string]any, len(t))
		for k, item := range t {
			obj[k] = bsonToAny(item)
		}
		return obj
	default:
		return t
	}
}
