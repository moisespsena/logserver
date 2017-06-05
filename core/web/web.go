package web

import (
	"gopkg.in/macaron.v1"
	"github.com/moisespsena/logserver/core/types"
	"github.com/moisespsena/logserver/core/web/routes"
)

func Init(G *types.Global) *macaron.Macaron  {
	m := macaron.Classic()
	m.Use(macaron.Static("static", macaron.StaticOptions{
		Prefix: G.Route("static"),
		SkipLogging: true,
	}))

	m.Use(macaron.Renderer())

	routes.Init(G, m)

	return m
}
