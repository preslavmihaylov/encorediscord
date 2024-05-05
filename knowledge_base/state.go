package knowledgebase

import "encore.dev/storage/sqldb"

var db = sqldb.NewDatabase("knowledge_base", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})
