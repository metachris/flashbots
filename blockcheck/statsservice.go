package blockcheck

// import (
// 	"time"

// 	"github.com/go-redis/redis/v8"
// )

// type StatsService struct {
// 	rdb              *redis.Client
// 	MinerErrorsToday map[string]*MinerErrorCounter // key is miner hash as hex string
// 	// MinerErrorsThisWeek map[string]*MinerErrorCounter // key is miner hash as hex string
// 	LastUpdate time.Time
// }

// func NewMinerStatsService() StatsService {
// 	rdb := redis.NewClient(&redis.Options{
// 		Addr: "localhost:6379",
// 	})

// 	service := StatsService{
// 		rdb:              rdb,
// 		MinerErrorsToday: make(map[string]*MinerErrorCounter),
// 	}

// 	// Load error counts from redis

// 	return service
// }

// func (s *StatsService) AddErrors(minerHash string, minerName string, block int64, errors ErrorCounts) {
// 	_, found := s.MinerErrorsToday[minerHash]
// 	if !found {
// 		s.MinerErrorsToday[minerHash] = &MinerErrorCounter{
// 			MinerHash: minerHash,
// 			MinerName: minerName,
// 			Blocks:    make(map[int64]bool),
// 		}
// 	}

// 	s.MinerErrorsToday[minerHash].AddErrorCounts(block, errors)
// }
