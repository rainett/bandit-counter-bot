package main

import (
	"bandit-counter-bot/internal/config"
	"bandit-counter-bot/internal/handlers"
	"bandit-counter-bot/internal/repository"
	"bandit-counter-bot/internal/service"
	"database/sql"
	"log"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	cfg := config.Load()
	db, err := sql.Open("sqlite3", cfg.DBPath)
	if err != nil {
		log.Fatal(err)
	}
	if err := repository.Migrate(db); err != nil {
		log.Fatal("db migration failed:", err)
	}
	defer db.Close()

	userStatsRepo := repository.NewUserStatsRepo(db)
	settingsRepo := repository.NewSettingsRepo(db)

	slotService := service.NewSlotService(userStatsRepo, settingsRepo)
	settingsService := service.NewSettingsService(settingsRepo)

	b, err := gotgbot.NewBot(cfg.BotToken, nil)
	if err != nil {
		log.Fatal(err)
	}

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		Error: func(b *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
			log.Println("an error occurred while handling update:", err.Error())
			return ext.DispatcherActionNoop
		},
		MaxRoutines: ext.DefaultMaxRoutines,
	})
	updater := ext.NewUpdater(dispatcher, &ext.UpdaterOpts{})

	dispatcher.AddHandler(handlers.GetSlotHandler(slotService))
	dispatcher.AddHandler(handlers.GetMeCommand(slotService))
	dispatcher.AddHandler(handlers.GetRichCommand(slotService))
	dispatcher.AddHandler(handlers.GetDebtorsCommand(slotService))
	dispatcher.AddHandler(handlers.GetSettingsCommand(settingsService))
	dispatcher.AddHandler(handlers.GetPrizeClassicCommand(settingsService))
	dispatcher.AddHandler(handlers.GetPrizeThreeInARowCommand(settingsService))
	dispatcher.AddHandler(handlers.GetPrizeLemonsCommand(settingsService))

	err = updater.StartPolling(b, &ext.PollingOpts{
		DropPendingUpdates:    false,
		EnableWebhookDeletion: true,
	})
	if err != nil {
		log.Fatal("failed to start polling:", err.Error())
		return
	}
	log.Println("Bot has been started...", "bot_username", b.User.Username)
	updater.Idle()
}
