package core

import (
	"context"
	"errors"
	"sync"

	"github.com/Jx2f/ViaGenshin/internal/config"
	"github.com/Jx2f/ViaGenshin/internal/mapper"
	"github.com/Jx2f/ViaGenshin/pkg/alg"
)

type Service struct {
	keys    *Keys
	mapping *mapper.Mapping

	mu      sync.RWMutex
	servers map[config.Protocol]*Server

	ctx       context.Context
	ctxCancel context.CancelFunc
	stopping  sync.WaitGroup

	AoiManager *alg.AoiManager
	TerrainMap map[uint32]*Terrain
}

func NewService() *Service {
	s := new(Service)
	s.servers = make(map[config.Protocol]*Server)
	s.ctx, s.ctxCancel = context.WithCancel(context.Background())
	s.stopping = sync.WaitGroup{}
	if config.GetConfig().TerrainCollect {
		s.InitTerrain()
	}
	return s
}

func (s *Service) Start() error {
	var err error
	s.keys, err = NewKeysFromConfig(config.GetConfig().Keys)
	if err != nil {
		return err
	}
	s.mapping, err = mapper.NewMappingFromConfig(config.GetConfig().Protocols)
	if err != nil {
		return err
	}
	for v := range config.GetConfig().Endpoints.Mapping {
		server, err := NewServer(s, config.GetConfig().Endpoints, v)
		if err != nil {
			return err
		}
		go func() {
			s.stopping.Add(1)
			if err2 := server.Start(s.ctx); err2 != nil {
				err = errors.New(err.Error() + "\n" + err2.Error())
			}
			s.stopping.Done()
		}()
		s.servers[v] = server
	}
	select {
	case <-s.ctx.Done():
	}
	return err
}

func (s *Service) Stop() error {
	s.ctxCancel()
	if config.GetConfig().TerrainCollect {
		s.SaveTerrain()
	}
	return nil
}
