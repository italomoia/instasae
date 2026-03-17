package main

import (
	"fmt"

	_ "github.com/aws/aws-sdk-go-v2/credentials"
	_ "github.com/aws/aws-sdk-go-v2/service/s3"
	_ "github.com/caarlos0/env/v11"
	_ "github.com/go-chi/chi/v5"
	_ "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5"
	_ "github.com/redis/go-redis/v9"
)

func main() {
	fmt.Println("instasae starting...")
}
