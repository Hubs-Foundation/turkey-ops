package internal

import (
	"context"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

var PgxPool *pgxpool.Pool

func MakePgxPool() {
	p, err := pgxpool.Connect(context.Background(), Cfg.DBconn)
	if err != nil {
		Logger.Error("Unable to connect to database: " + err.Error())
	}
	PgxPool = p
}

// func UpsertHubStatus(hubId, status string) error {
// 	_, err := PgxPool.Exec(
// 		context.Background(),
// 		fmt.Sprintf(`INSERT into hubs (hub_id, status) values ('%v','%v') ON CONFLICT DO UPDATE`, hubId, status),
// 	)
// 	if err != nil {
// 		Logger.Error(err.Error())
// 		return err
// 	}
// 	return nil
// }

var OrchDb *pgxpool.Pool

func MakeOrchDb() {
	pool, err := pgxpool.Connect(context.Background(), Cfg.DBconn+"/turkeyorch")
	if err != nil {
		Logger.Error("Unable to connect to database: " + err.Error())
	}
	OrchDb = pool

	lockKey := "888"
	locked := false
	for i := 0; i < 10; i++ {
		err := pool.QueryRow(context.Background(), "SELECT pg_try_advisory_lock($1)", lockKey).Scan(&locked)
		if err != nil {
			Logger.Sugar().Fatalf("Failed to try to acquire lock: %v", err)
		}
		if locked {
			break
		}

		// Sleep before retrying
		Logger.Info("Another instance is running migrations, waiting...")
		time.Sleep(6 * time.Second)
	}
	if !locked {
		Logger.Fatal("Failed to acquire lock for migrations")
	}
	defer pool.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, lockKey)

	//run migration
	pool.Exec(context.Background(), `CREATE TABLE IF NOT EXISTS migration (key TEXT);`)
	scripts := getMigrationsScriptsArray()
	var migrationsCnt int
	err = pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM table_name").Scan(&migrationsCnt)
	if err != nil {
		Logger.Sugar().Fatalf("Failed to query row count: %v\n", err)
	}

	for i := migrationsCnt; i < len(scripts); i++ {
		sql := scripts[i]
		res, err := pool.Exec(context.Background(), sql)
		if err != nil {
			Logger.Sugar().Fatalf("Failed to execute %s: %v", sql, err)
		}
		Logger.Sugar().Infof("executed: <%v>, result: %v", sql, string(res))
	}

	//

}
func getMigrationsScriptsArray() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS hubs (hub_id INT PRIMARY KEY);`,
		`ALTER TABLE hubs 
			ADD COLUMN IF NOT EXISTS account_id INT, 
			ADD COLUMN IF NOT EXISTS fxa_sub TEXT, 
			ADD COLUMN IF NOT EXISTS hub_id INT PRIMARY KEY,
			ADD COLUMN IF NOT EXISTS name TEXT,
			ADD COLUMN IF NOT EXISTS tier TEXT,
			ADD COLUMN IF NOT EXISTS status TEXT,
			ADD COLUMN IF NOT EXISTS email TEXT,
			ADD COLUMN IF NOT EXISTS subdomain TEXT,
			ADD COLUMN IF NOT EXISTS inserted_at timestamp with time zone DEFAULT timezone('UTC', CURRENT_TIMESTAMP);`,
	}
}

// func getMigrationsScripts() map[string]string {
// 	return map[string]string{
// 		"0_hubs_table": `CREATE TABLE IF NOT EXISTS hubs (hub_id INT PRIMARY KEY);`,
// 		"1_hubs_table_columns": `ALTER TABLE hubs
// 			ADD COLUMN IF NOT EXISTS account_id INT,
// 			ADD COLUMN IF NOT EXISTS fxa_sub TEXT,
// 			ADD COLUMN IF NOT EXISTS hub_id INT PRIMARY KEY,
// 			ADD COLUMN IF NOT EXISTS name TEXT,
// 			ADD COLUMN IF NOT EXISTS tier TEXT,
// 			ADD COLUMN IF NOT EXISTS status TEXT,
// 			ADD COLUMN IF NOT EXISTS email TEXT,
// 			ADD COLUMN IF NOT EXISTS subdomain TEXT;`,
// 	}
// }

var DashboardDb *pgxpool.Pool

func MakeDashboardDb() {
	p, err := pgxpool.Connect(context.Background(), Cfg.DBconn+"/dashboard")
	if err != nil {
		Logger.Error("Unable to connect to database: " + err.Error())
	}
	DashboardDb = p
}

func MakeDbs() {
	MakePgxPool()
	MakeOrchDb()
	MakeDashboardDb()
}
