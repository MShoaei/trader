package server

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/MShoaei/trader/internals"
	"github.com/adshao/go-binance/v2"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func (s *Server) CreateWatchdog(c *gin.Context) {
	data := []struct {
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
	log.Info(user.Watchdogs)

	created := make([]string, 0, len(data))
	exists := make([]string, 0, len(data))
	for _, d := range data {
		if _, ok := user.GetWatchdog(d.Symbol, d.Interval); ok {
			exists = append(exists, d.Symbol)
			continue
		}
		interruptCh := make(chan os.Signal, 1)
		w := &internals.Watchdog{
			Symbol:     d.Symbol,
			Interval:   d.Interval,
			Risk:       d.Risk,
			Leverage:   d.Leverage,
			Commission: d.Commission * 0.01,
			Demo:       d.Demo,

			InterruptCh: interruptCh,
		}
		wsKlineHandler, errHandler, err := w.Watch(user.Client)
		if err != nil {
			log.Error(err)
		}
		go func() {
			id := NewWatchdogID(w.Symbol, w.Interval)
			user.Watchdogs[id] = w

			t := time.NewTicker(23 * time.Hour)
			defer t.Stop()
		loop:
			doneC, stopC, err := binance.WsKlineServe(w.Symbol, w.Interval, wsKlineHandler, errHandler)
			if err != nil {
				log.Error(err)
				return
			}
			w.StopC = stopC

			select {
			case <-doneC:
				return
			case <-t.C:
				close(stopC)
				goto loop
			case <-w.InterruptCh:
				close(stopC)
				delete(user.Watchdogs, id)
			}
		}()
		created = append(created, d.Symbol)
	}
	c.JSON(http.StatusOK, gin.H{
		"created": created,
		"exists":  exists,
	})
}

func (s *Server) StopWatchdog(c *gin.Context) {
	symbol := c.Param("symbol")
	interval := c.Param("interval")
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

	ok, err := user.DeleteWatchdog(symbol, interval)
	if err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		success(c, http.StatusOK, "watchdog does not exist")
		return
	}
	message := fmt.Sprintf("watchdog %s deleted", string(NewWatchdogID(symbol, interval)))
	success(c, http.StatusOK, message)
}

func (s *Server) GetWatchdog(c *gin.Context) {
	symbol := c.Param("symbol")
	interval := c.Param("interval")
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
	w, ok := user.GetWatchdog(symbol, interval)
	if !ok {
		fail(c, http.StatusNotFound, fmt.Errorf("watchdog not found"))
		return
	}
	c.JSON(http.StatusOK, w)
}

func (s *Server) GetWatchdogAnalysis(c *gin.Context) {
	symbol := c.Param("symbol")
	interval := c.Param("interval")
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

	w, ok := user.GetWatchdog(symbol, interval)
	if !ok {
		fail(c, http.StatusNotFound, fmt.Errorf("watchdog not found"))
	}
	c.JSON(http.StatusOK, w.Report())
}
