package router

import (
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func DefaultRouter() *gin.Engine {

	gin.DefaultWriter = log.Logger.Level(zerolog.InfoLevel)
	gin.DefaultErrorWriter = log.Logger.Level(zerolog.ErrorLevel)

	r := gin.Default()
	return r
}
