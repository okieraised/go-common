package http_server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/okieraised/go-common/config"
	"github.com/okieraised/go-common/constants"
	"github.com/okieraised/go-common/infrastructures/logger"
	"github.com/okieraised/go-common/middlewares"
	"github.com/spf13/viper"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func StartServer(appService *api.AppService, wsService *ws.WSService, tr trace.TracerProvider, quit chan os.Signal) {
	logger.GetDefault().Info("Started initializing api server")

	gin.SetMode(viper.GetString(config.ServerMode))

	router := gin.New()
	router.Use(gin.Recovery())

	router.Use(cors.New(cors.Config{
		AllowOrigins: []string{constants.CORSAllowAllOrigins},
		AllowMethods: []string{http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodGet, http.MethodDelete},
		AllowHeaders: []string{constants.HeaderAccessControlAllowHeaders, constants.HeaderOrigin, constants.HeaderAccept,
			constants.HeaderXRequestedWith, constants.HeaderContentType, constants.HeaderAuthorization, constants.HeaderXAPIKey},
		ExposeHeaders:    []string{constants.HeaderContentLength},
		AllowCredentials: true,
	}))

	timeout := constants.DefaultHTTPTimeout
	if viper.GetInt(config.APITimeoutDuration) > 0 {
		timeout = time.Duration(viper.GetInt(config.APITimeoutDuration)) * time.Second
	}
	router.NoRoute(middlewares.NoRouteMW())
	router.Use(middlewares.RequestIDMW())

	//rootRouter := New(appService, wsService)
	//rootRouter.InitWSRouters(router, tr)

	router.Use(
		middlewares.RequestTimeoutMW(timeout),
		gzip.Gzip(gzip.DefaultCompression),
		middlewares.RecoveryMW(),
		middlewares.RequestLoggingMW(logger.GetDefault().Logger),
		middlewares.ResponseHashMW(),
	)
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	//rootRouter.InitAPIRouters(router, tr)

	serverPort := constants.DefaultHTTPPort
	if viper.GetString(config.ServerHttpPort) != "" {
		serverPort = viper.GetInt(config.ServerHttpPort)
	}

	serverAddr := fmt.Sprintf("0.0.0.0:%d", serverPort)
	srv := &http.Server{
		Addr:    serverAddr,
		Handler: router,
	}

	go func() {
		var err error

		if viper.GetBool(config.ServerEnableTLS) {
			logger.GetDefault().Info("TLS enabled")
			err = srv.ListenAndServeTLS(viper.GetString(config.ServerCertFile), viper.GetString(config.ServerKeyFile))
		} else {
			logger.GetDefault().Info("TLS disabled")
			err = srv.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.GetDefault().Error(err.Error())
			os.Exit(1)
		}
	}()
	logger.GetDefault().Info(fmt.Sprintf("HTTP server started, listening on: %s", serverAddr))

	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(viper.GetInt(config.ServerGracefulShutdownWait))*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.GetDefault().Error(fmt.Sprintf("Failed to shut down HTTP server: %s", err.Error()))
	}

	select {
	case <-ctx.Done():
		logger.GetDefault().Info("Stopped receiving new requests")
	}
}
