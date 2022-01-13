package internal

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
)

var PgxPool *pgxpool.Pool

func MakePgxPool() {
	p, err := pgxpool.Connect(context.Background(), Cfg.DBconn)
	if err != nil {
		logger.Error("Unable to connect to database: " + err.Error())
	}
	PgxPool = p
}
