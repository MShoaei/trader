package server

import (
	"context"
	"log"

	"github.com/adshao/go-binance/v2"
	"github.com/gin-gonic/gin"
)

type Server struct {
	api   *gin.Engine
	info  *binance.ExchangeInfo
	index map[string]int
}

func NewServer() *Server {
	s := &Server{}
	info, err := binance.NewClient("", "").
		NewExchangeInfoService().
		Do(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	s.info = info
	s.index = make(map[string]int, len(s.info.Symbols))
	for i, symbol := range info.Symbols {
		s.index[symbol.Symbol] = i
		symbol.LotSizeFilter()
	}

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
