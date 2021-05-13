package server

import (
	"fmt"
	"net/http"
	"os"

	"github.com/MShoaei/trader/internals"
	"github.com/gin-gonic/gin"
)

func (s *Server) CreateWatchdog(c *gin.Context) {
	data := struct {
		Symbol     string
		Interval   string
		Risk       float64
		Commission float64
		Leverage   int
		Demo       bool
	}{}
	if err := c.BindJSON(&data); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	if _, ok := internals.GetWatchdog(data.Symbol, data.Interval); ok {
		fail(c, http.StatusConflict, fmt.Errorf("watchdog already exists"))
		return
	}
	interruptCh := make(chan os.Signal, 1)
	w := internals.Watchdog{
		Client:     s.client,
		Symbol:     data.Symbol,
		Interval:   data.Interval,
		Risk:       data.Risk,
		Leverage:   data.Leverage,
		Commission: data.Commission,
		Demo:       data.Demo,

		InterruptCh: interruptCh,
	}
	go w.Watch()
	success(c, http.StatusCreated, "")
}

func (s *Server) StopWatchdog(c *gin.Context) {
	symbol := c.Param("symbol")
	interval := c.Param("interval")
	ok, err := internals.DeleteWatchdog(symbol, interval)
	if err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		success(c, http.StatusOK, "watchdog does not exist")
		return
	}
	message := fmt.Sprintf("watchdog %s deleted", string(internals.NewID(symbol, interval)))
	success(c, http.StatusOK, message)
}

func (s *Server) GetWatchdog(c *gin.Context) {
	symbol := c.Param("symbol")
	interval := c.Param("interval")
	w, ok := internals.GetWatchdog(symbol, interval)
	if !ok {
		fail(c, http.StatusNotFound, fmt.Errorf("watchdog not found"))
		return
	}
	c.JSON(http.StatusOK, w)
}

func (s *Server) GetWatchdogAnalysis(c *gin.Context) {
	symbol := c.Param("symbol")
	interval := c.Param("interval")
	w, ok := internals.GetWatchdog(symbol, interval)
	if !ok {
		fail(c, http.StatusNotFound, fmt.Errorf("watchdog not found"))
	}
	c.JSON(http.StatusOK, w.Report())
}
