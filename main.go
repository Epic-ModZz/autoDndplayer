package main

/*
here is the filepath claude
PCL
PCL/go.mod
PCL/.DS_Store
PCL/go.sum
PCL/.env
PCL/db
PCL/db/SQL_CharStats
PCL/db/SQL_CharStats/mutableDataUpdateC.go
PCL/db/SQL_CharStats/seeder_helpers.go
PCL/db/SQL_CharStats/seeder_races.go
PCL/db/SQL_CharStats/seeder_spells.go
PCL/db/SQL_CharStats/runtimeDataStorage
PCL/db/SQL_CharStats/runtimeDataStorage/.DS_Store
PCL/db/SQL_CharStats/seeder_classes.go
PCL/db/SQL_CharStats/.DS_Store
PCL/db/SQL_CharStats/seeder_backgrounds.go
PCL/db/SQL_CharStats/tableStructs
PCL/db/SQL_CharStats/tableStructs/Mutable
PCL/db/SQL_CharStats/tableStructs/Mutable/LevelUpDecisions.go
PCL/db/SQL_CharStats/tableStructs/Mutable/roleplay.go
PCL/db/SQL_CharStats/tableStructs/Mutable/schemes.go
PCL/db/SQL_CharStats/tableStructs/Mutable/characters.go
PCL/db/SQL_CharStats/tableStructs/Immutable
PCL/db/SQL_CharStats/tableStructs/Immutable/subrace.go
PCL/db/SQL_CharStats/tableStructs/Immutable/subclass.go
PCL/db/SQL_CharStats/tableStructs/Immutable/bastion.go
PCL/db/SQL_CharStats/tableStructs/Immutable/armor.go
PCL/db/SQL_CharStats/tableStructs/Immutable/proficiency.go
PCL/db/SQL_CharStats/tableStructs/Immutable/boons.go
PCL/db/SQL_CharStats/tableStructs/Immutable/spells.go
PCL/db/SQL_CharStats/tableStructs/Immutable/background.go
PCL/db/SQL_CharStats/tableStructs/Immutable/race.go
PCL/db/SQL_CharStats/tableStructs/Immutable/raceFeatures.go
PCL/db/SQL_CharStats/tableStructs/Immutable/eldritchInvocations.go
PCL/db/SQL_CharStats/tableStructs/Immutable/class.go
PCL/db/SQL_CharStats/tableStructs/Immutable/language.go
PCL/db/SQL_CharStats/tableStructs/Immutable/monster.go
PCL/db/SQL_CharStats/tableStructs/Immutable/subclassFeatures.go
PCL/db/SQL_CharStats/tableStructs/Immutable/classFeatures.go
PCL/db/SQL_CharStats/tableStructs/Immutable/multiclassSpellSlots.go
PCL/db/SQL_CharStats/tableStructs/Immutable/conditions.go
PCL/db/SQL_CharStats/tableStructs/Immutable/fightingStyles.go
PCL/db/SQL_CharStats/tableStructs/Immutable/item.go
PCL/db/SQL_CharStats/tableStructs/Immutable/featFeatures.go
PCL/db/SQL_CharStats/tableStructs/Immutable/weapons.go
PCL/db/SQL_CharStats/tableStructs/Immutable/feat.go
PCL/db/SQL_CharStats/tableStructs/.DS_Store
PCL/db/SQL_CharStats/tableStructs/RawData
PCL/db/SQL_CharStats/tableStructs/RawData/.DS_Store
PCL/db/SQL_CharStats/tableStructs/RawData/Data
PCL/db/SQL_CharStats/tableStructs/RawData/Data/monsters.json
PCL/db/SQL_CharStats/tableStructs/RawData/Data/class-artificer.json
PCL/db/SQL_CharStats/tableStructs/RawData/Data/.DS_Store
PCL/db/SQL_CharStats/tableStructs/RawData/Data/class-cleric.json
PCL/db/SQL_CharStats/tableStructs/RawData/Data/races.json
PCL/db/SQL_CharStats/tableStructs/RawData/Data/class-barbarian.json
PCL/db/SQL_CharStats/tableStructs/RawData/Data/class-sorcerer.json
PCL/db/SQL_CharStats/tableStructs/RawData/Data/class-bard.json
PCL/db/SQL_CharStats/tableStructs/RawData/Data/class-fighter.json
PCL/db/SQL_CharStats/tableStructs/RawData/Data/class-wizard.json
PCL/db/SQL_CharStats/tableStructs/RawData/Data/backgrounds.json
PCL/db/SQL_CharStats/tableStructs/RawData/Data/class-monk.json
PCL/db/SQL_CharStats/tableStructs/RawData/Data/class-rogue.json
PCL/db/SQL_CharStats/tableStructs/RawData/Data/class-ranger.json
PCL/db/SQL_CharStats/tableStructs/RawData/Data/class-warlock.json
PCL/db/SQL_CharStats/tableStructs/RawData/Data/fighting_styles_and_EldritchInvocations.json
PCL/db/SQL_CharStats/tableStructs/RawData/Data/class-paladin.json
PCL/db/SQL_CharStats/tableStructs/RawData/Data/class-druid.json
PCL/db/SQL_CharStats/tableStructs/RawData/Data/spells.json
PCL/db/SQL_CharStats/tableStructs/RawData/Data/feats&boons.json
PCL/db/SQL_CharStats/knowledgeSourceMigration.go
PCL/db/SQL_CharStats/Seed.go
PCL/db/SQL_CharStats/files
PCL/db/SQL_CharStats/files/.DS_Store
PCL/db/SQL_CharStats/files.zip
PCL/db/SQL_CharStats/seeder_feats.go
PCL/db/SQL_CharStats/seeder_monsters.go
PCL/db/SQL_CharStats/SQL_Init.go
PCL/db/.DS_Store
PCL/db/Dynamic_Qdrant
PCL/bot
PCL/bot/queryExe.go
PCL/bot/.DS_Store
PCL/bot/shouldRespond.go
PCL/bot/respondClassifier.go
PCL/bot/respondLoop.go
PCL/bot/Dmcommands.go
PCL/bot/Memorywriter.go
PCL/bot/summarizer.go
PCL/bot/LevelupPipeline.go
PCL/bot/channelClassifier.go
PCL/bot/infoGatherer.go
PCL/bot/xpPipeline.go
PCL/bot/responseGenerator.go
PCL/bot/LLMCall.go
PCL/bot/botInit.go
PCL/bot/batchFilterer.go
PCL/main.go

*/

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
