package server

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/MShoaei/trader/internals"
	"github.com/adshao/go-binance/v2"
	"github.com/gin-gonic/gin"
)

type User struct {
	Client    *binance.Client
	Watchdogs map[ID]*internals.Watchdog
	APIKey    string
	APISecret string
}

var allUsers = map[string]*User{}

func (s *Server) Register(c *gin.Context) {
	data := struct {
		Key    string
		Secret string
	}{}
	if err := c.BindJSON(&data); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	if _, ok := allUsers[data.Key]; ok {
		fail(c, http.StatusConflict, fmt.Errorf("API key already exists"))
		return
	}
	allUsers[data.Key] = &User{
		Watchdogs: make(map[ID]*internals.Watchdog, 100),
		APIKey:    data.Key,
		APISecret: data.Secret,
	}
}

type ID string

func NewWatchdogID(symbol string, interval string) ID {
	return ID(symbol + "-" + interval)
}

func (id ID) GetSymbol() string {
	return strings.Split(string(id), "-")[0]
}

func (id ID) GetInterval() string {
	return strings.Split(string(id), "-")[1]
}

func (u *User) GetWatchdog(symbol string, interval string) (*internals.Watchdog, bool) {
	id := NewWatchdogID(symbol, interval)
	w, ok := u.Watchdogs[id]
	return w, ok
}

func (u *User) DeleteWatchdog(symbol string, interval string) (bool, error) {
	id := NewWatchdogID(symbol, interval)
	w, ok := u.Watchdogs[id]
	if !ok {
		return false, nil
	}
	t := time.NewTimer(2 * time.Second)
	select {
	case w.StopC <- struct{}{}:
		break
	case <-t.C:
		if !t.Stop() {
			<-t.C
		}
		return false, fmt.Errorf("failed to stop the watchdog")
	}
	delete(u.Watchdogs, id)
	return true, nil
}
