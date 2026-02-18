package main

import (
	"bandit-counter-bot/internal/config"
	"bandit-counter-bot/internal/handlers"
	"bandit-counter-bot/internal/repository"
	"bandit-counter-bot/internal/service"
	"bandit-counter-bot/migrations"
	"database/sql"
	"log"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	tghandlers "github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	db, err := sql.Open("sqlite3", cfg.DBPath)
	if err != nil {
		log.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if err := repository.Migrate(db, migrations.FS); err != nil {
		log.Fatal("db migration failed:", err)
	}
	defer db.Close()

	userStatsRepo := repository.NewUserStatsRepo(db)
	settingsRepo := repository.NewSettingsRepo(db)

	slotMessageCache := service.NewSlotMessageCache()
	_ = slotMessageCache.LoadFromFile("slot_cache.json")

	authService := service.NewAuthService(cfg.DevIDs, settingsRepo)
	slotService := service.NewSlotService(userStatsRepo, settingsRepo, slotMessageCache)
	settingsService := service.NewSettingsService(settingsRepo, authService)
	statsService := service.NewStatsService(userStatsRepo)
	resetService := service.NewResetService(userStatsRepo, authService)

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
	dispatcher.AddHandler(handlers.GetCleanCommand(slotService))
	dispatcher.AddHandler(tghandlers.NewCommand("me", slotService.HandleMeCommand))
	dispatcher.AddHandler(tghandlers.NewCommand("stats", statsService.HandleStatsCommand))
	dispatcher.AddHandler(tghandlers.NewCommand("settings", settingsService.HandleSettingsCommand))
	dispatcher.AddHandler(tghandlers.NewCommand("reset", resetService.HandleResetCommand))
	dispatcher.AddHandler(tghandlers.NewCommand("help", slotService.HandleHelpCommand))
	dispatcher.AddHandler(tghandlers.NewCallback(callbackquery.Prefix("stats:"), statsService.HandleStatsCallback))
	dispatcher.AddHandler(tghandlers.NewCallback(callbackquery.Prefix("settings:"), settingsService.HandleSettingsCallback))
	dispatcher.AddHandler(tghandlers.NewCallback(callbackquery.Prefix("reset:"), resetService.HandleResetCallback))

	err = updater.StartPolling(b, &ext.PollingOpts{
		DropPendingUpdates:    false,
		EnableWebhookDeletion: true,
	})
	if err != nil {
		log.Fatal("failed to start polling:", err.Error())
	}
	log.Println("Bot has been started...", "bot_username", b.User.Username)

	updater.Idle()

	log.Println("shutting down, saving slot cache...")
	if err := slotMessageCache.SaveToFile("slot_cache.json"); err != nil {
		log.Println("failed to save slot cache:", err)
	}
}
