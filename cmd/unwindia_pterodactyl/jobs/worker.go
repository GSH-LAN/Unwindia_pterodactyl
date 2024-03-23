package jobs

import (
	"context"
	"errors"
	"fmt"
	"github.com/GSH-LAN/Unwindia_common/src/go/config"
	"github.com/GSH-LAN/Unwindia_common/src/go/matchservice"
	"github.com/GSH-LAN/Unwindia_common/src/go/messagebroker"
	steam_api_token "github.com/GSH-LAN/Unwindia_common/src/go/steam-api-token"
	"github.com/GSH-LAN/Unwindia_common/src/go/workitemLock"
	"github.com/GSH-LAN/Unwindia_pterodactyl/cmd/unwindia_pterodactyl/database"
	"github.com/GSH-LAN/Unwindia_pterodactyl/cmd/unwindia_pterodactyl/pterodactyl"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/dvsekhvalnov/jose2go/base64url"
	"github.com/gammazero/workerpool"
	jsoniter "github.com/json-iterator/go"
	"github.com/modern-go/reflect2"
	"github.com/parkervcp/crocgodyl"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/sync/semaphore"
	"strconv"
	"time"
)

type Worker struct {
	ctx            context.Context
	db             database.DatabaseClient
	pteroClient    pterodactyl.Client
	workerpool     *workerpool.WorkerPool
	matchPublisher message.Publisher
	semaphore      *semaphore.Weighted
	jobLock        workitemLock.WorkItemLock
	config         config.ConfigClient
	baseTopic      string
	steamApiClient *steam_api_token.Client
}

const (
	mappedServerPasswordEnv     = "ServerPassword"
	mappedServerMgmtPasswordEnv = "ServerPasswordMgmt"
	mappedGameServerToken       = "GameServerToken"
	mappedServerTvAddressPort   = "ServerTvAddressPort"
)

func NewWorker(ctx context.Context, db database.DatabaseClient, pool *workerpool.WorkerPool, pteroClient pterodactyl.Client, matchPublisher message.Publisher, config config.ConfigClient, baseTopic string, steamApiClient *steam_api_token.Client) *Worker {
	w := Worker{
		ctx:            ctx,
		db:             db,
		pteroClient:    pteroClient,
		workerpool:     pool,
		matchPublisher: matchPublisher,
		semaphore:      semaphore.NewWeighted(int64(1)),
		jobLock:        workitemLock.NewMemoryWorkItemLock(),
		config:         config,
		baseTopic:      baseTopic,
		steamApiClient: steamApiClient,
	}
	return &w
}

func (w *Worker) StartWorker(interval time.Duration) error {
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ticker.C:
			go w.process()
		case <-w.ctx.Done():
			return w.ctx.Err()
		}
	}
}

func (w *Worker) process() {
	log.Trace().Msg("start job processing")

	if !w.semaphore.TryAcquire(1) {
		log.Warn().Msg("Skip processing, semaphore already acquired")
		return
	}
	defer w.semaphore.Release(1)

	newJobFilter := bson.M{"status": database.JobStatusNew}
	newJobs, err := w.db.List(w.ctx, newJobFilter)
	if err != nil {
		log.Err(err).Msg("error retrieving new jobs from database")
		return
	}
	log.Trace().Interface("jobs", newJobs).Msg("retrieved new jobs")

	for _, job := range newJobs {
		err := w.processJob(w.ctx, job)
		if err != nil {
			log.Error().Err(err).Str("jobId", job.ID.String()).Msg("Error processing job")
		}
	}

	log.Trace().Msg("finished job processing")
}

func (w *Worker) processJob(ctx context.Context, job *database.Job) error {
	log.Trace().Msg("start job processing")

	if !w.lockJob(job.ID) {
		return errors.New("cannot lock job")
	}
	defer w.unlockJob(job.ID)

	switch job.Action {
	case database.ActionCreate:
		var address string
		var port string

		gsTemplate, err := w.config.GetGameServerTemplateForMatch(job.MatchInfo)
		if err != nil {
			return err
		}

		server, err := w.pteroClient.FindExistingServerForMatch(job.MatchId)
		if err == nil || !reflect2.IsNil(server) {
			// no server found, we can determine a new server
			log.Warn().Interface("server", server).Str("jobId", job.ID.String()).Msg("got an existing server for this match")
		} else {
			server, err = w.pteroClient.GetBestFittingSuspendedServer(gsTemplate.EggId)
			if err != nil {
				return err
			}
			log.Info().Interface("server", server).Str("jobId", job.ID.String()).Msg("got server for job")
		}

		allocation, err := w.pteroClient.GetNodeAllocation(server.Node, server.Allocation)
		if err != nil {
			return err
		}

		address = allocation.IP
		port = strconv.Itoa(int(allocation.Port))

		details := crocgodyl.ServerDetailsDescriptor{
			ExternalID:  job.MatchId,
			Name:        gsTemplate.ServerNamePrefix + job.MatchId,
			User:        gsTemplate.UserId,
			Description: fmt.Sprintf(pterodactyl.ServerUseDescription, job.MatchId),
		}
		startup := crocgodyl.ServerStartupDescriptor{
			Startup:     gsTemplate.DefaultStartup,
			Environment: gsTemplate.Environment,
			Egg:         gsTemplate.EggId,
			Image:       gsTemplate.DefaultDockerImage,
			SkipScripts: server.StartupDescriptor().SkipScripts,
		}

		if gsTokenMapping, ok := gsTemplate.EnvironmentMapping[mappedGameServerToken]; ok {
			token, err := w.GetGameServerTokenForMatch(gsTemplate.SteamApiTokenAppId, job.MatchInfo)
			if err != nil {
				log.Error().Err(err).Msg("Error fetching GameServerTokenForMatch")
			}
			startup.Environment[gsTokenMapping] = token
		}

		if gsTvPortMapping, ok := gsTemplate.EnvironmentMapping[mappedServerTvAddressPort]; ok {
			startup.Environment[gsTvPortMapping] = int(allocation.Port) + gsTemplate.TvPortOffset
		}

		serverIdentifier := server.Identifier
		err = w.pteroClient.ReuseExistingServer(server.ID, serverIdentifier, details, startup)
		if err != nil {
			log.Error().Err(err).Str("jobid", job.ID.String()).Int("server.id", server.ID).Msg("Error reusing server")
			return err
		}

		job.ServerId = server.ID
		job.Status = database.JobStatusInProgress

		var pass string
		var servermgmtpass string

		serverPassEnv, ok := gsTemplate.EnvironmentMapping[mappedServerPasswordEnv]
		if !ok {
			serverPassEnv = mappedServerPasswordEnv
		}

		serverMgmtPassEnv, ok := gsTemplate.EnvironmentMapping[mappedServerMgmtPasswordEnv]
		if !ok {
			serverMgmtPassEnv = mappedServerMgmtPasswordEnv
		}

		pass, _ = server.StartupDescriptor().Environment[serverPassEnv].(string)
		servermgmtpass, _ = server.StartupDescriptor().Environment[serverMgmtPassEnv].(string)

		job.MatchInfo.ServerPassword = pass
		job.MatchInfo.ServerAddress = fmt.Sprintf("%s:%s", address, port)
		job.MatchInfo.ServerPasswordMgmt = servermgmtpass
		_, err = w.db.UpdateJob(ctx, job)
		if err != nil {
			log.Error().Err(err).Str("jobid", job.ID.String()).Int("server.id", server.ID).Msg("Error updating job")
			return err
		}

		msg := messagebroker.Message{
			Type:    messagebroker.MessageTypeUpdated,
			SubType: messagebroker.UNWINDIA_MATCH_SERVER_READY.String(),
			Data:    &job.MatchInfo,
		}

		if j, err := jsoniter.Marshal(msg); err != nil {
			log.Warn().Err(err).Msg("Error while marshalling message")
			return err
		} else {
			msg := message.Message{
				Payload: j,
			}

			err = w.matchPublisher.Publish(w.baseTopic, &msg)
			if err != nil {
				log.Error().Err(err).Msg("Error publishing to messagebroker")
				return err
			}
		}

		job.Status = database.JobStatusFinished

		_, err = w.db.UpdateJob(ctx, job)
		if err != nil {
			log.Error().Err(err).Str("jobid", job.ID.String()).Int("server.id", server.ID).Msg("Error updating job")
			return err
		}

	case database.ActionDelete:
		if job.ExecAfter != nil && time.Now().After(*job.ExecAfter) || job.ExecAfter == nil {
			err := w.pteroClient.SuspendServer(job.ServerId)
			if err != nil {
				return err
			}

			job.Status = database.JobStatusFinished

			_, err = w.db.UpdateJob(ctx, job)
			if err != nil {
				log.Error().Err(err).Str("jobid", job.ID.String()).Int("server.id", job.ServerId).Msg("Error updating job")
				return err
			}
		}
	}

	log.Trace().Msg("finished job processing")
	return nil
}

func (w *Worker) lockJob(id primitive.ObjectID) bool {
	if err := w.jobLock.Lock(w.ctx, id.String(), nil); err != nil {
		log.Warn().Str("job", id.String()).Err(err).Msg("Error locking contest")
		return false
	}
	log.Trace().Str("job", id.String()).Msg("Locked contest")
	return true
}

func (w *Worker) unlockJob(id primitive.ObjectID) bool {
	if err := w.jobLock.Unlock(w.ctx, id.String()); err != nil {
		log.Warn().Str("contest", id.String()).Err(err).Msg("Error unlocking contest")
		return false
	}
	log.Trace().Str("contest", id.String()).Msg("Unlocked contest")
	return true
}

// GetGameServerToken fetches a Steam api token for a gameserver
func (w *Worker) GetGameServerTokenForMatch(appId int, info matchservice.MatchInfo) (string, error) {
	// generate serverId based on info.ServerAddress base64
	serverId := base64url.Encode([]byte(info.ServerAddress))

	// fetch token from steam api
	token, err := w.steamApiClient.GetSteamApiToken(w.ctx, appId, serverId)
	if err != nil {
		return "", err
	}

	return token, nil
}
