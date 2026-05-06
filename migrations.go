package jobhuntos

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS
