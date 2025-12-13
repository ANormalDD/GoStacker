package bootstrap

import (
	"GoStacker/pkg/config"
	"GoStacker/pkg/db/mysql"
	rdb "GoStacker/pkg/db/redis"
	"GoStacker/pkg/logger"
	"GoStacker/pkg/monitor"
	"GoStacker/pkg/utils"
	"fmt"
)

// InitAll initializes config/logger/mysql/redis/monitor and returns a cleanup func.
// configPath is path to a YAML config file. If empty, falls back to default config.Init().
func InitAll(configPath string) (cleanup func(), err error) {
	if configPath != "" {
		if err = config.InitFromFile(configPath); err != nil {
			return nil, err
		}
	} else {
		if err = config.Init(); err != nil {
			return nil, err
		}
	}

	if err = logger.Init(config.Conf.LogConfig); err != nil {
		return nil, fmt.Errorf("init logger failed: %w", err)
	}

	if err = mysql.Init(config.Conf.MySQLConfig); err != nil {
		return nil, fmt.Errorf("init mysql failed: %w", err)
	}

	if err = rdb.Init(config.Conf.RedisConfig); err != nil {
		mysql.Close()
		return nil, fmt.Errorf("init redis failed: %w", err)
	}

	utils.SetJWTConfig(config.Conf.JWTConfig)
	monitor.InitMonitor()

	cleanup = func() {
		mysql.Close()
		rdb.Close()
		// flush logger
		_ = logger.L().Sync()
	}
	return cleanup, nil
}
