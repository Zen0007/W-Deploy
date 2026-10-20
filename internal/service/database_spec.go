package service

import (
	"time"

	"github.com/moby/moby/api/types/container"
)

type dbSpec struct {
	Image      string
	DefaultVer string
	Port       int
	VolumePath string
	// EnvFunc menerima identity yang sudah di-resolve agar env
	// dan connection URI selalu konsisten.
	EnvFunc     func(user, dbName, password string) []string
	HealthCheck *container.HealthConfig
}

var dbSpecs = map[DBEngine]dbSpec{
	DBPostgres: {
		Image:      "postgres",
		DefaultVer: "17-alpine",
		Port:       5432,
		VolumePath: "/var/lib/postgresql/data",
		EnvFunc: func(user, dbName, pw string) []string {
			return []string{
				"POSTGRES_PASSWORD=" + pw,
				"POSTGRES_USER=" + user,
				"POSTGRES_DB=" + dbName,
			}
		},
		HealthCheck: &container.HealthConfig{
			Test:     []string{"CMD-SHELL", "pg_isready -U \"$POSTGRES_USER\" -d \"$POSTGRES_DB\""},
			Interval: 10 * time.Second,
			Timeout:  5 * time.Second,
			Retries:  5,
		},
	},
	DBMySQL: {
		Image:      "mysql",
		DefaultVer: "8.4",
		Port:       3306,
		VolumePath: "/var/lib/mysql",
		EnvFunc: func(user, dbName, pw string) []string {
			env := []string{
				"MYSQL_ROOT_PASSWORD=" + pw,
				"MYSQL_DATABASE=" + dbName,
			}
			// MySQL menolak MYSQL_USER=root.
			if user != "" && user != "root" {
				env = append(env, "MYSQL_USER="+user, "MYSQL_PASSWORD="+pw)
			}
			return env
		},
		HealthCheck: &container.HealthConfig{
			Test:     []string{"CMD-SHELL", `mysqladmin ping -h 127.0.0.1 -u root -p"$MYSQL_ROOT_PASSWORD" --silent`},
			Interval: 10 * time.Second,
			Timeout:  5 * time.Second,
			Retries:  10, // MySQL 8.4 first-init agak lama
		},
	},
	DBMongo: {
		Image:      "mongo",
		DefaultVer: "7",
		Port:       27017,
		VolumePath: "/data/db",
		EnvFunc: func(user, dbName, pw string) []string {
			return []string{
				"MONGO_INITDB_ROOT_USERNAME=" + user,
				"MONGO_INITDB_ROOT_PASSWORD=" + pw,
				"MONGO_INITDB_DATABASE=" + dbName,
			}
		},
		HealthCheck: &container.HealthConfig{
			Test: []string{"CMD-SHELL",
				`mongosh --quiet --eval 'db.adminCommand({ping:1}).ok' ` +
					`-u "$MONGO_INITDB_ROOT_USERNAME" ` +
					`-p "$MONGO_INITDB_ROOT_PASSWORD" ` +
					`--authenticationDatabase admin | grep -q 1`},
			Interval: 10 * time.Second,
			Timeout:  5 * time.Second,
			Retries:  5,
		},
	},
	DBRedis: {
		Image:      "redis",
		DefaultVer: "7-alpine",
		Port:       6379,
		VolumePath: "/data",
		EnvFunc: func(_, _, pw string) []string {
			// redis-server tidak butuh env, tapi redis-cli di healthcheck
			// membaca REDISCLI_AUTH otomatis.
			return []string{"REDISCLI_AUTH=" + pw}
		},
		HealthCheck: &container.HealthConfig{
			Test:     []string{"CMD", "redis-cli", "ping"},
			Interval: 10 * time.Second,
			Timeout:  5 * time.Second,
			Retries:  5,
		},
	},
}
