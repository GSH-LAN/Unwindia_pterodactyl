package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/GSH-LAN/Unwindia_common/src/go/config"
	"github.com/GSH-LAN/Unwindia_common/src/go/matchservice"
	"github.com/GSH-LAN/Unwindia_common/src/go/messagebroker"
	"github.com/GSH-LAN/Unwindia_pterodactyl/cmd/unwindia_pterodactyl/database"
	"github.com/GSH-LAN/Unwindia_pterodactyl/cmd/unwindia_pterodactyl/environment"
	"github.com/GSH-LAN/Unwindia_pterodactyl/cmd/unwindia_pterodactyl/jobs"
	"github.com/GSH-LAN/Unwindia_pterodactyl/cmd/unwindia_pterodactyl/messagequeue"
	"github.com/GSH-LAN/Unwindia_pterodactyl/cmd/unwindia_pterodactyl/pterodactyl"
	"github.com/GSH-LAN/Unwindia_pterodactyl/cmd/unwindia_pterodactyl/router"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/gammazero/workerpool"
	"github.com/gin-gonic/gin"
	"github.com/meysamhadeli/problem-details"
	"github.com/mitchellh/mapstructure"
	"github.com/parkervcp/crocgodyl"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	swaggerui "github.com/swaggest/swgui/v4"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type Server struct {
	ctx            context.Context
	env            *environment.Environment
	config         config.ConfigClient
	workerpool     *workerpool.WorkerPool
	subscriber     *messagequeue.Subscriber
	matchPublisher message.Publisher
	messageChan    chan *messagebroker.Message
	lock           sync.Mutex
	dbClient       database.DatabaseClient
	stop           chan struct{}
	router         *gin.Engine
	pteroClient    pterodactyl.Client
}

func NewServer(ctx context.Context, env *environment.Environment, cfgClient config.ConfigClient, matchPublisher message.Publisher, wp *workerpool.WorkerPool) (*Server, error) {
	messageChan := make(chan *messagebroker.Message)

	subscriber, err := messagequeue.NewSubscriber(ctx, env, messageChan)
	if err != nil {
		return nil, err
	}

	dbClient, err := database.NewClient(ctx, env)
	if err != nil {
		return nil, err
	}

	pteroClient, err := pterodactyl.NewClient(&env.PteroApplicationApiURL, &env.PteroClientApiURL, env.PteroApplicationApiToken, env.PteroClientApiToken, wp, env.PteroFetchInterval)
	if err != nil {
		return nil, err
	}

	jobHandler := jobs.NewWorker(ctx, dbClient, wp, pteroClient, matchPublisher, cfgClient)

	srv := Server{
		ctx:            ctx,
		env:            env,
		config:         cfgClient,
		workerpool:     wp,
		subscriber:     subscriber,
		messageChan:    messageChan,
		lock:           sync.Mutex{},
		dbClient:       dbClient,
		stop:           make(chan struct{}),
		router:         router.DefaultRouter(),
		pteroClient:    pteroClient,
		matchPublisher: matchPublisher,
	}

	go func() {
		_ = jobHandler.StartWorker(env.JobsProcessInterval)
	}()

	return &srv, nil
}

func (s *Server) Start() error {
	s.setupRouter()
	go func() {
		_ = s.router.Run(fmt.Sprintf(":%d", s.env.HTTPPort))
	}()

	s.subscriber.StartConsumer()
	for {
		select {
		case <-s.stop:
			log.Info().Msg("Stopping processing, server stopped")
			return nil
		case message := <-s.messageChan:
			s.workerpool.Submit(func() {
				s.messageHandler(message)
			})
		}
	}
}

func (s *Server) Stop() error {
	log.Info().Msgf("Stopping server")
	close(s.stop)
	return fmt.Errorf("server Stopped")
}

func (s *Server) messageHandler(message *messagebroker.Message) {
	log.Info().Interface("message", message).Msg("received message")

	match := matchservice.MatchInfo{}
	err := mapstructure.WeakDecode(message.Data, &match)
	if err != nil {
		log.Error().Err(err).Msg("Error decoding match")
		return
	}

	log.Info().Interface("match", match).Msg("Received match")

	// in this case we need to get a server for that match
	// so we need to create a new job
	var jobIds []primitive.ObjectID
	if message.SubType == messagebroker.UNWINDIA_MATCH_READY_ALL.String() {
		log.Info().Str("id", match.Id).Msg("Match is ready to get a server, creating job")

		matchId := match.Id
		if s.env.UseMatchServiceId {
			matchId = match.MsID
		}

		job := database.Job{
			Action:    database.ActionCreate,
			Status:    database.JobStatusNew,
			MatchId:   matchId,
			Game:      match.Game,
			Slots:     int(match.PlayerAmount),
			ExecAfter: nil,
			MatchInfo: match,
		}

		jobId, err := s.dbClient.CreateJob(s.ctx, &job)
		if err != nil {
			// TODO: some retry stuff we need :(
			log.Error().Err(err).Str("id", match.Id).Msg("Error creating job for match")
			return
		}

		jobIds = []primitive.ObjectID{jobId}

		log.Debug().Str("jobId", jobId.String()).Str("id", match.Id).Msg("Created job for match")
	}

	// Match is done, we can delete/suspend the server after some waiting time
	if message.SubType == messagebroker.UNWINDIA_MATCH_FINISHED.String() {
		log.Info().Str("id", match.Id).Msg("Match is finished, creating delete job")

		matchId := match.Id
		if s.env.UseMatchServiceId {
			matchId = match.MsID
		}

		// find existing matchinfo to determine server
		existingMatchInfo, err := s.dbClient.GetMatchInfo(s.ctx, match.Id)
		if err != nil {
			log.Error().Err(err).Str("id", match.Id).Msg("error finding exising matchinfo for delete job")
			return
		}

		if len(existingMatchInfo.JobIds) == 0 {
			log.Warn().Str("matchid", matchId).Msg("We got a delete job but didn't have a previous job for this!!!!!!!!!!!!!!!11111einself")
			return
		}

		existingJob, err := s.dbClient.GetJob(s.ctx, existingMatchInfo.JobIds[len(existingMatchInfo.JobIds)].String())
		if err != nil {
			log.Error().Err(err).Str("id", match.Id).Msg("error finding exising matchinfo for delete job")
			return
		}

		job := database.Job{
			Action:    database.ActionDelete,
			Status:    database.JobStatusNew,
			MatchId:   matchId,
			Game:      match.Game,
			Slots:     int(match.PlayerAmount),
			ServerId:  existingJob.ServerId,
			ExecAfter: nil,
			MatchInfo: matchservice.MatchInfo{},
		}

		jobId, err := s.dbClient.CreateJob(s.ctx, &job)
		if err != nil {
			// TODO: some retry stuff we need :(
			log.Error().Err(err).Str("id", match.Id).Msg("Error creating delete job for match")
			return
		}

		jobIds = append(existingMatchInfo.JobIds, jobId)

		log.Debug().Str("jobId", jobId.String()).Str("id", match.Id).Msg("Created delete job for match")
	}

	matchEntry := database.MatchInfo{
		Id:        match.Id,
		MatchInfo: match,
		JobIds:    jobIds,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = s.dbClient.UpsertMatchInfo(s.ctx, &matchEntry)
	if err != nil {
		log.Error().Err(err).Interface("message", *message)
	}
}

func (s *Server) setupRouter() {
	// GinErrorHandler middleware for handle problem details error on gin
	s.router.Use(func(c *gin.Context) {
		c.Next()

		for _, err := range c.Errors {

			// add custom map problem details here...

			if _, err := problem.ResolveProblemDetails(c.Writer, c.Request, err); err != nil {
				log.Error().Err(err).Msg("gin error")
			}
		}
	})

	s.router.GET("/swagger", gin.WrapH(swaggerui.NewHandler("Unwindia Pterodactyl", "/api/unwindia_pterodactyl.yaml", "/")))

	internal := s.router.Group("/api/internal")
	//internal.GET("/health", gin.WrapF(handlers.NewJSONHandlerFunc(health.Health, nil)))
	internal.GET("/metrics", gin.WrapH(promhttp.Handler()))

	//serverApi := s.router.Group("/api/v1/server")
	//serverApi.POST("/")

	v1Api := s.router.Group("/api/v1")
	v1Api.POST("/jobs", s.handleCreateJob)
	v1Api.POST("/preinstall/:game/:amount", s.handlePreinstall)

	v1Api.GET("/match/:matchid/:template", s.handleMatchTemplate)
}

func (s *Server) handleCreateJob(ctx *gin.Context) {
	var job database.Job
	err := ctx.BindJSON(&job)
	if err != nil {
		log.Err(err).Msg("failed binding job")
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("%v", err),
		})
		return
	}

	log.Info().Interface("job", job).Msg("creating Job")
	createdJobId, err := s.dbClient.CreateJob(ctx, &job)
	if err != nil {
		// TODO: respond with RFC7807 error response
		ctx.Err()
	}

	ctx.JSON(http.StatusCreated, gin.H{"jobid": createdJobId})
}

func (s *Server) handlePreinstall(ctx *gin.Context) {
	amountParam := ctx.Param("amount")
	amount, err := strconv.Atoi(amountParam)
	if err != nil {
		log.Err(err).Msg("failed binding job")
		ctx.Error(err)
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("%v", err),
		})
		return
	}

	gameParam := ctx.Param("game")

	gsTemplate, err := s.config.GetGameServerTemplate(gameParam)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": err,
		})
		return
	}

	log.Info().Int("amount", amount).Str("game", gameParam).Msg("creating preinstall job")

	var createdJobs []string
	var batchError error
	for i := 1; i <= amount; i++ {
		log.Info().Int("no", i).Msg("creating Server")
		createdServer, err := s.pteroClient.PreinstallServer(gsTemplate)
		// TODO: evaluate crogocyl apierror and log real error informations (e.g. fucked up environment variables)
		if err != nil {
			// TODO: respond with RFC7807 error response
			if errVal, ok := err.(*crocgodyl.ApiError); ok {
				for _, apiErr := range errVal.Errors {
					log.Error().Err(apiErr).Int("no", i).Msg("error creating server")
				}
			}
			log.Error().Err(err).Int("no", i).Msg("error creating server")
			batchError = errors.Join(batchError, err)
			continue
		}
		createdJobs = append(createdJobs, strconv.Itoa(createdServer.ID))
	}

	if batchError != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": batchError.Error()})
	} else {
		ctx.JSON(http.StatusCreated, gin.H{"servers": createdJobs})
	}
}

func (s *Server) handleMatchTemplate(ctx *gin.Context) {
	matchid := ctx.Param("matchid")
	template := ctx.Param("template")

	log.Info().Str("matchid", matchid).Str("template", template).Msg("creating template for match")
}
