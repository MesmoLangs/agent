Implement a new server feature end-to-end in the Mesmo Go + Fiber + GORM codebase.

Procedure:
1. GORM Model (model/database/) — UUID primary keys, auto timestamps, cascade foreign keys
2. Request/Response DTO (model/handlers/) — Req/Resp structs, camelCase JSON tags
3. Handler (handlers/{feature}/) — use common.HandleRequest wrapper, file-level logger vars, database.DB for queries
4. Register Route in main.go — use existing group, naming: /feature/action.entity
5. Add Constants to config/constants.go if needed
6. Verify — `go build ./...`, route in main.go, AutoMigrate includes new model, no fmt.Println

$ARGUMENTS
