package server

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) GetTotalAnalysis(c *gin.Context) {
	u, ok := c.Get(identityKey)
	if !ok {
		fail(c, http.StatusInternalServerError, fmt.Errorf("this should never happen :)"))
		return
	}

	user, ok := u.(*User)
	if !ok {
		fail(c, http.StatusInternalServerError, fmt.Errorf("this should never happen either :)"))
		return
	}

	resp := gin.H{}
	for id, w := range user.Watchdogs {
		resp[string(id)] = w.Report()
	}
	c.JSON(http.StatusOK, resp)
}
