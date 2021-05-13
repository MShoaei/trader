package server

import "github.com/gin-gonic/gin"

func (s *Server) InitAPI() {
	r := gin.Default()

	r.GET("/positions")
	r.GET("/position/:symbol/:interval")

	r.GET("/config", s.GetConfig)
	r.POST("/config", s.SetConfig)

	r.GET("/watchdogs")
	r.GET("/watchdog/:symbol/:interval", s.GetWatchdog)
	r.POST("/watchdog", s.CreateWatchdog)
	// r.PATCH("/watchdog/:symbol/:interval")
	r.DELETE("/watchdog/:symbol/:interval", s.StopWatchdog)

	r.GET("/watchdog/:symbol/:interval/analysis", s.GetWatchdogAnalysis)

	s.api = r
}
