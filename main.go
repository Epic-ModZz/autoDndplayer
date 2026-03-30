package main


import (
	"PCL/bot"
	db "PCL/db/SQL_CharStats"
	"log"
	"os"
	"time"
)

func main() {
	botToken, ok := os.LookupEnv("botToken")
	if !ok {
		log.Fatal("No Token")
	}

	db.Init("db/SQL_CharStats/runtimeDataStorage/game.db")

	bot.BotToken = botToken

	// Persona env vars — used by the OOC response generator to impersonate
	// a real player. These do not affect the character name used for D&D.
	bot.PlayerName = os.Getenv("PLAYER_NAME")
	bot.PlayerAge = os.Getenv("PLAYER_AGE")
	bot.PlayerJob = os.Getenv("PLAYER_JOB")
	bot.PlayerLocation = os.Getenv("PLAYER_LOCATION")
	bot.PlayerDetails = os.Getenv("PLAYER_DETAILS")
	bot.PlayerTimezone = os.Getenv("PLAYER_TIMEZONE")

	// Design and seed the character if this is a fresh database.
	var count int
	db.DB.QueryRow("SELECT COUNT(*) FROM characters").Scan(&count)
	if count == 0 {
		if err := bot.DesignAndSeedCharacter(); err != nil {
			log.Fatal(err)
		}
	}

	// Always sync CharacterName from the DB so the heuristic gate
	// (containsCharacterName), the LLM classifier, and the response generator
	// all use the same name — whatever the LLM chose, not the env var.
	// The env var CHARACTER_NAME is kept as a fallback for the very first
	// startup tick before DesignAndSeedCharacter returns.
	var dbCharName string
	if err := db.DB.QueryRow("SELECT name FROM characters WHERE id = 1").Scan(&dbCharName); err == nil && dbCharName != "" {
		bot.CharacterName = dbCharName
		log.Printf("CharacterName synced from DB: %q", dbCharName)
	} else {
		// DB sync failed — fall back to env var and log loudly so it's obvious.
		bot.CharacterName = os.Getenv("CHARACTER_NAME")
		log.Printf("CharacterName DB sync failed, using env var: %q", bot.CharacterName)
	}

	for {
		bot.Run()
		log.Println("Bot disconnected, waiting 30s before reconnect...")
		time.Sleep(30 * time.Second)
	}
}
