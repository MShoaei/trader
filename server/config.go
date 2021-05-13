package server

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"

	"github.com/adshao/go-binance/v2"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func (s *Server) SetConfig(c *gin.Context) {
	data := struct {
		Key    string
		Secret string
	}{}
	if err := c.BindJSON(&data); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	s.client = binance.NewClient(data.Key, data.Secret)
	success(c, http.StatusOK, "")
}

func (s *Server) GetConfig(c *gin.Context) {
	if s.client == nil {
		fail(c, http.StatusNotFound, fmt.Errorf("client is not initialized"))
		return
	}
	data := struct {
		Key string
	}{s.client.APIKey}
	c.JSON(http.StatusOK, data)
}

func (s *Server) SetProxy(c *gin.Context) {
	data := struct {
		Proxy string
	}{}
	if s.client == nil {
		fail(c, http.StatusServiceUnavailable, fmt.Errorf("client is not initialized"))
		return
	}
	if err := c.BindJSON(data); err != nil {
		fail(c, http.StatusBadRequest, err)
	}
	if data.Proxy == "" {
		s.client.HTTPClient.Transport = http.DefaultTransport
	} else {
		proxyURL, _ := url.Parse(data.Proxy)
		s.client.HTTPClient.Transport = &http.Transport{
			Proxy:           http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		websocket.DefaultDialer.Proxy = http.ProxyURL(proxyURL)
	}
	success(c, http.StatusOK, "")
}
