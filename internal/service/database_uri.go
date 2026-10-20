package service

import "fmt"

// buildConnectionURI membangun string koneksi sesuai engine.
func buildConnectionURI(engine DBEngine, user, pass, host string, port int, db string) string {
	switch engine {
	case DBPostgres:
		return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?sslmode=disable",
			user, pass, host, port, db)
	case DBMySQL:
		return fmt.Sprintf("mysql://%s:%s@%s:%d/%s", user, pass, host, port, db)
	case DBMongo:
		return fmt.Sprintf("mongodb://%s:%s@%s:%d/%s?authSource=admin",
			user, pass, host, port, db)
	case DBRedis:
		return fmt.Sprintf("redis://:%s@%s:%d/0", pass, host, port)
	default:
		return ""
	}
}
