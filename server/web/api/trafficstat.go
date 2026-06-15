package api

import (
	"server/torr"

	"github.com/gin-gonic/gin"
)

// trafficStat godoc
//
//	@Summary		Server-wide traffic counters
//	@Description	Returns session-scoped traffic totals in bytes: download/upload from peers
//	@Description	(summed over active torrents) and served to players over HTTP (LAN).
//
//	@Tags			API
//
//	@Produce		json
//	@Success		200	{object}	object	"download, upload, served (bytes)"
//	@Router			/trafficstat [get]
func trafficStat(c *gin.Context) {
	download, upload, served := torr.GlobalTraffic()
	c.JSON(200, gin.H{
		"download": download,
		"upload":   upload,
		"served":   served,
	})
}
