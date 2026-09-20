package database

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ConnectPostgres(ctx context.Context, databaseUrl string) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	config, err := pgxpool.ParseConfig(databaseUrl)
	if err != nil {
		log.Printf("Unable to parse URL: %v\n", databaseUrl)
		return nil, err
	}

	config.MaxConns = 20
	config.MinConns = 5
	config.MaxConnIdleTime = 5 * time.Second
	config.MaxConnLifetime = 1 * time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		log.Println("Unable to create connection")
		return nil, err
	}

	err = pool.Ping(ctx)
	if err != nil {
		log.Printf("Unable to ping database: %v", err)
		pool.Close()
		return nil, err
	}

	log.Println("Successfully connected to PostgreSQL database")

	return pool, nil
}
