package server

import (
	"time"

	"github.com/adshao/go-binance/v2"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

var identityKey = "id"
var key = []byte("4ZGR$FyK5ykkjtCZGxTH&$XjHiK9vS8V")

func NewJWTMiddleware() *jwt.GinJWTMiddleware {
	// the jwt middleware
	authMiddleware, err := jwt.New(&jwt.GinJWTMiddleware{
		Realm:       "PrettyRealm",
		Key:         key,
		Timeout:     30 * 24 * time.Hour,
		MaxRefresh:  24 * time.Hour,
		IdentityKey: identityKey,
		PayloadFunc: func(data interface{}) jwt.MapClaims {
			if v, ok := data.(*User); ok {
				return jwt.MapClaims{
					identityKey: v.APIKey,
				}
			}
			return jwt.MapClaims{}
		},
		IdentityHandler: func(c *gin.Context) interface{} {
			claims := jwt.ExtractClaims(c)
			return allUsers[claims[identityKey].(string)]
		},
		Authenticator: func(c *gin.Context) (interface{}, error) {
			data := struct {
				Key    string
				Secret string
			}{}
			if err := c.BindJSON(&data); err != nil || data.Key == "" {
				return "", jwt.ErrMissingLoginValues
			}
			if user, ok := allUsers[data.Key]; ok && user.APISecret == data.Secret {
				user.Client = binance.NewClient(data.Key, data.Secret)
				return user, nil
			}
			return "", jwt.ErrForbidden
		},
		Authorizator: func(data interface{}, c *gin.Context) bool {
			return true
		},
		Unauthorized: func(c *gin.Context, code int, message string) {
			c.JSON(code, gin.H{
				"message": message,
			})
		},
		TokenLookup:   "header: Authorization",
		TokenHeadName: "Bearer",
		TimeFunc:      time.Now,
	})

	if err != nil {
		log.Fatal("JWT Error:" + err.Error())
	}

	// When you use jwt.New(), the function is already automatically called for checking,
	// which means you don't need to call it again.
	errInit := authMiddleware.MiddlewareInit()

	if errInit != nil {
		log.Fatal("authMiddleware.MiddlewareInit() Error:" + errInit.Error())
	}

	return authMiddleware
}
