package internal

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v4/pgxpool"
)

// type Pg struct {
// 	pool *pgxpool.Pool
// }

var PgxPool *pgxpool.Pool

func MakePgxPool() {
	p, err := pgxpool.Connect(context.Background(), cfg.DBconn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Unable to connect to database:", err)
		os.Exit(1)
	}
	PgxPool = p
}
