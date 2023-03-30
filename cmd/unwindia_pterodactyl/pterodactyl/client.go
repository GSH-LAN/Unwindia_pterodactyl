package pterodactyl

import (
	"errors"
	"fmt"
	"github.com/GSH-LAN/Unwindia_common/src/go/config"
	"github.com/gammazero/workerpool"
	"github.com/parkervcp/crocgodyl"
	"github.com/rs/zerolog/log"
	"github.com/segmentio/ksuid"
	"golang.org/x/sync/semaphore"
	"net/url"
	"sync"
	"time"
)

const (
	ServerPreInstallDescription = "unwindia_preinstall"
	ServerUseDescription        = "Unwindia_Match_%s"
)

type Client interface {
	GetBestFittingSuspendedServer(eggId int) (*crocgodyl.AppServer, error)
	ReuseExistingServer(serverId int, serverIdentifier string, serverDetails crocgodyl.ServerDetailsDescriptor, serverStartup crocgodyl.ServerStartupDescriptor) error
	CreateNewServer() (*crocgodyl.AppServer, error)
	DeleteServer(serverId int) error
	SuspendServer() error
	PreinstallServer(gsTemplate *config.GamerServerConfigTemplate) (*crocgodyl.AppServer, error)
	FindExistingServerForMatch(matchid string) (*crocgodyl.AppServer, error)
	GetNodeAllocation(nodeId, allocationId int) (*crocgodyl.Allocation, error)
}

type ClientImpl struct {
	applicationClient *crocgodyl.Application
	clientClient      *crocgodyl.Client
	workerpool        *workerpool.WorkerPool

	pteroServers []pteroServerLock
	serverLock   sync.RWMutex
	semaphore    semaphore.Weighted
}

type pteroServerLock struct {
	*crocgodyl.AppServer
	locked bool
}

func NewClient(apiAppUrl, apiClientUrl *url.URL, apiAppToken, apiClientToken string, pool *workerpool.WorkerPool, pterodactylFetchInterval time.Duration) (Client, error) {
	appClient, err := crocgodyl.NewApp(apiAppUrl.String(), apiAppToken)
	if err != nil {
		return nil, err
	}

	clientClient, err := crocgodyl.NewClient(apiClientUrl.String(), apiClientToken)
	if err != nil {
		return nil, err
	}

	c := ClientImpl{
		applicationClient: appClient,
		clientClient:      clientClient,
		workerpool:        pool,
		pteroServers:      nil,
		serverLock:        sync.RWMutex{},
		semaphore:         semaphore.Weighted{},
	}

	go c.startBackgroundFetching(pterodactylFetchInterval)
	return &c, nil
}

func (c *ClientImpl) GetBestFittingSuspendedServer(eggId int) (*crocgodyl.AppServer, error) {
	servers, err := c.applicationClient.GetServers()
	if err != nil {
		return nil, err
	}
	log.Trace().Interface("servers", servers).Msg("Got servers to determine best fitting server")

	for _, server := range servers {
		if server.Suspended && server.Egg == eggId {
			return server, nil
		}
	}

	return nil, fmt.Errorf("no server found")
}

func (c *ClientImpl) ReuseExistingServer(serverId int, serverIdentifier string, serverDetails crocgodyl.ServerDetailsDescriptor, serverStartup crocgodyl.ServerStartupDescriptor) error {
	_, err := c.applicationClient.UpdateServerDetails(serverId, serverDetails)
	if err != nil {
		return err
	}

	//_, err = c.applicationClient.UpdateServerStartup(serverId, serverStartup)
	//if err != nil {
	//	return err
	//}

	err = c.applicationClient.UnsuspendServer(serverId)
	if err != nil {
		return err
	}

	time.Sleep(time.Millisecond * 500)

	err = c.clientClient.SetServerPowerState(serverIdentifier, "start")
	if err != nil {
		return err
	}

	return nil
}

func (c *ClientImpl) CreateNewServer() (*crocgodyl.AppServer, error) {
	//TODO implement me
	panic("implement me")
}

func (c *ClientImpl) DeleteServer(serverId int) error {
	//TODO implement me
	panic("implement me")
}

func (c *ClientImpl) SuspendServer() error {
	//TODO implement me
	panic("implement me")
}

// TODO: rename this
func (c *ClientImpl) fetchPteroStuff() {

	c.serverLock.Lock()
	defer c.serverLock.Unlock()

	// TODO: maybe we need to check here for pagination in case of too many servers... :(
	servers, err := c.applicationClient.GetServers()
	if err != nil {
		log.Error().Err(err).Msg("fetchPteroStuff() error retrieving servers")
		return
	}

	log.Trace().Interface("server", servers).Msg("retrieved current servers")

	var serverLockList []pteroServerLock
	for _, server := range servers {
		serverLockList = append(serverLockList, pteroServerLock{AppServer: server})
	}

	c.pteroServers = serverLockList
}

func (c *ClientImpl) checkInstallStateAndSusped() {
	c.serverLock.Lock()
	defer c.serverLock.Unlock()

	for i, server := range c.pteroServers {
		if !server.Suspended && server.ExternalID == "" && server.Description == ServerPreInstallDescription && server.Container.Installed == 1 {
			log.Info().Interface("server", server).Msg("found server which could be suspended!")
			suspendedServer, err := c.suspendServer(server.AppServer)
			if err != nil {
				log.Error().Err(err).Int("server.id", server.ID).Msg("error suspending preinstalled server")
			}

			c.pteroServers[i].AppServer = suspendedServer
		}
	}
}

func (c *ClientImpl) suspendServer(server *crocgodyl.AppServer) (*crocgodyl.AppServer, error) {
	data := server.DetailsDescriptor()
	data.ExternalID = ""
	data.Name = ksuid.New().String()
	data.Description = ""
	server, err := c.applicationClient.UpdateServerDetails(server.ID, *data)
	if err != nil {
		return nil, err
	}

	err = c.applicationClient.SuspendServer(server.ID)
	if err != nil {
		return nil, err
	}

	return c.applicationClient.GetServer(server.ID)
}

func (c *ClientImpl) startBackgroundFetching(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ticker.C:
			c.fetchPteroStuff()
			c.checkInstallStateAndSusped()
		}
	}
}

func (c *ClientImpl) PreinstallServer(gsTemplate *config.GamerServerConfigTemplate) (*crocgodyl.AppServer, error) {
	// TODO: check if gsTemplate.DefaultStartup command is empty and if so, fetch the egg's default command

	createServerDescriptor := crocgodyl.CreateServerDescriptor{
		ExternalID:    "",
		Name:          ksuid.New().String(),
		Description:   ServerPreInstallDescription,
		User:          gsTemplate.UserId,
		Egg:           gsTemplate.EggId,
		DockerImage:   gsTemplate.DefaultDockerImage,
		Startup:       gsTemplate.DefaultStartup,
		Environment:   gsTemplate.Environment,
		SkipScripts:   false,
		OOMDisabled:   gsTemplate.Limits.OOMDisabled,
		Limits:        &gsTemplate.Limits,
		FeatureLimtis: crocgodyl.FeatureLimits{},
		Allocation:    nil,
		Deploy: &crocgodyl.DeployDescriptor{
			Locations:   []int{gsTemplate.LocationId},
			DedicatedIP: false,
			PortRange:   []string{},
		},
		StartOnCompletion: false,
	}

	server, err := c.applicationClient.CreateServer(createServerDescriptor)
	if err != nil {
		return nil, err
	}

	return server, nil
}

func (c *ClientImpl) FindExistingServerForMatch(matchid string) (*crocgodyl.AppServer, error) {
	return c.applicationClient.GetServerExternal(matchid)
}

func (c *ClientImpl) GetNodeAllocation(nodeId, allocationId int) (*crocgodyl.Allocation, error) {
	allocations, err := c.applicationClient.GetNodeAllocations(nodeId)
	if err != nil {
		return nil, err
	}

	for _, allocation := range allocations {
		if allocation.ID == allocationId {
			return allocation, nil
		}
	}

	return nil, errors.New("allocation not found")
}
