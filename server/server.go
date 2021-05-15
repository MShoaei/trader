package server

import (
	"github.com/gin-gonic/gin"
)

type Server struct {
	api *gin.Engine
}

func NewServer() *Server {
	s := &Server{}
	s.InitAPI()

	return s
}

func (s *Server) Run() {
	s.api.Run()
}

func fail(c *gin.Context, code int, err error) {
	c.JSON(code, gin.H{
		"error": err.Error(),
	})
}

func success(c *gin.Context, code int, message string) {
	if message == "" {
		c.JSON(code, gin.H{})
		return
	}
	c.JSON(code, gin.H{
		"message": message,
	})
}
