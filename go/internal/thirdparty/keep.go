// Package thirdparty 把并行开发会用到的依赖留在 go.mod 里。
// 模块 agent 直接 import 这些路径，不要再改 go.mod，也不要 go get。
package thirdparty

import (
	_ "github.com/elastic/go-elasticsearch/v8"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5"
	_ "github.com/redis/go-redis/v9"
	_ "github.com/twmb/franz-go/pkg/kgo"
	_ "go.mongodb.org/mongo-driver/mongo"
	_ "gopkg.in/yaml.v3"
)
