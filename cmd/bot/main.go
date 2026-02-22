package main

import (
	"database/sql"
	"log"
	"time"

	"bandit-counter-bot/internal/cache"
	"bandit-counter-bot/internal/config"
	"bandit-counter-bot/internal/handlers"
	"bandit-counter-bot/internal/repository"
	"bandit-counter-bot/internal/scheduler"
	"bandit-counter-bot/internal/service"
	"bandit-counter-bot/migrations"

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

	slotMessageCache := cache.NewSlotMessageCache()
	if err := slotMessageCache.LoadFromFile("slot_cache.json"); err != nil {
		log.Println("failed to load slot cache:", err)
	}

	cleaner := service.NewMessageCleaner(slotMessageCache)
	authService := service.NewAuthService(cfg.DevIDs, settingsRepo)
	slotService := service.NewSlotService(
		userStatsRepo,
		settingsRepo,
		slotMessageCache,
		cleaner,
	)
	settingsService := service.NewSettingsService(settingsRepo, authService)
	statsService := service.NewStatsService(userStatsRepo)
	resetService := service.NewResetService(userStatsRepo, authService)

	bot, err := gotgbot.NewBot(cfg.BotToken, nil)
	if err != nil {
		log.Fatal(err)
	}

	loc, err := time.LoadLocation("Europe/Uzhgorod")
	if err != nil {
		log.Printf("timezone Europe/Uzhgorod not found, using local: %v", err)
		loc = time.Local
	}

	sched := scheduler.NewScheduler(slotMessageCache, cleaner, bot, loc)
	sched.Start()
	defer sched.Stop()

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		Error: func(b *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
			log.Println("handler error:", err)
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

	err = updater.StartPolling(bot, &ext.PollingOpts{
		DropPendingUpdates:    false,
		EnableWebhookDeletion: true,
	})
	if err != nil {
		log.Fatal("failed to start polling:", err)
	}

	log.Println("Bot started", "username", bot.User.Username)

	updater.Idle()

	log.Println("shutting down, saving slot cache...")
	if err := slotMessageCache.SaveToFile("slot_cache.json"); err != nil {
		log.Println("failed to save slot cache:", err)
	}
}
