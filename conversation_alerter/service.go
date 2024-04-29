package conversationalerter

import "encore.dev/storage/sqldb"

var db = sqldb.NewDatabase("conversation_alerts", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})
