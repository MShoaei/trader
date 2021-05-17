package server

import "github.com/gin-gonic/gin"

func (s *Server) InitAPI() {
	r := gin.Default()

	authMiddleware := NewJWTMiddleware()
	r.POST("/register", s.Register)
	r.POST("/login", authMiddleware.LoginHandler)

	r.Use(authMiddleware.MiddlewareFunc())

	r.GET("/positions")
	r.GET("/position/:symbol/:interval")

	r.GET("/watchdogs")
	r.GET("/watchdog/:symbol/:interval", s.GetWatchdog)
	r.POST("/watchdog", s.CreateWatchdog)
	// r.PATCH("/watchdog/:symbol/:interval")
	r.DELETE("/watchdog/:symbol/:interval", s.StopWatchdog)

	r.GET("/watchdog/:symbol/:interval/analysis", s.GetWatchdogAnalysis)
	r.GET("/watchdogs/analysis", s.GetTotalAnalysis)

	s.api = r
}
