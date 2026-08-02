// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package main

import (
	"context"

	"github.com/gerege-systems/open-gerege-core/cmd/seed/seeders"
	"github.com/gerege-systems/open-gerege-core/core/config"
	"github.com/gerege-systems/open-gerege-core/core/constants"
	"github.com/gerege-systems/open-gerege-core/core/datasources/drivers"
	"github.com/gerege-systems/open-gerege-core/pkg/logger"
)

func init() {
	if err := config.InitializeAppConfig(); err != nil {
		logger.Fatal(err.Error(), logger.Fields{constants.LoggerCategory: constants.LoggerCategoryConfig})
	}
	logger.Info("configuration loaded", logger.Fields{constants.LoggerCategory: constants.LoggerCategoryConfig})
}

func main() {
	ctx := context.Background()
	pool, err := drivers.SetupPgxPostgres(ctx)
	if err != nil {
		logger.Panic(err.Error(), logger.Fields{constants.LoggerCategory: constants.LoggerCategorySeeder})
	}
	defer pool.Close()

	logger.Info("seeding...", logger.Fields{constants.LoggerCategory: constants.LoggerCategorySeeder})

	seeder := seeders.NewSeeder(pool)
	if err := seeder.UserSeeder(seeders.UserData); err != nil {
		logger.Panic(err.Error(), logger.Fields{constants.LoggerCategory: constants.LoggerCategorySeeder})
	}

	logger.Info("seeding success!", logger.Fields{constants.LoggerCategory: constants.LoggerCategorySeeder})
}
