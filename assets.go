package jobhuntos

import "embed"

//go:embed web/templates/*.html web/static/*
var Assets embed.FS
