package db

import (
	immutable "PCL/db/SQL_CharStats/tableStructs/Immutable"
	mutable "PCL/db/SQL_CharStats/tableStructs/Mutable"
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func Init(path string) {
	var err error
	DB, err = sql.Open("sqlite3", path)
	if err != nil {
		log.Fatal("Failed to open SQLite DB: ", err)
	}
	if err = DB.Ping(); err != nil {
		log.Fatal("Failed to ping SQLite DB: ", err)
	}
	createTables()
	RunKnowledgeSourceMigration()
	SeedReferenceData()
	log.Println("Database initialized")
}

func createTables() {
	schemas := []string{
		//schemes allow for the ai to more effectively remember its plots
		mutable.CharacterGoalsSchema(),
		mutable.CharacterSchemesSchema(),
		mutable.CharacterPublicPersonaSchema(),
		mutable.CharacterLiesSchema(),
		mutable.CharacterCorruptionArcSchema(),
		mutable.CharacterTriggerEventsSchema(),
		mutable.CharacterAgentsSchema(),

		mutable.CharacterClassResourcesSchema(),
		mutable.CharacterExhaustionSchema(),
		mutable.CharacterConcentrationSchema(),
		mutable.CharacterInspirationSchema(),
		mutable.CharacterPersonalitySchema(),
		mutable.CharacterBackstorySchema(),
		mutable.CharacterNotesSchema(),

		mutable.NpcDetailsSchema(),
		mutable.NpcSecretsSchema(),
		mutable.ChannelConfigSchema(),

		mutable.CharactersSchema(),
		mutable.CharacterClassesSchema(),
		mutable.CharacterSpellSlotsSchema(),
		mutable.CharacterSpellsKnownSchema(),
		mutable.CharacterFeaturesSchema(),
		mutable.CharacterFeatsSchema(),
		mutable.CharacterProficienciesSchema(),
		mutable.CharacterInventorySchema(),
		mutable.CharacterCurrencySchema(),
		mutable.CharacterConditionsSchema(),
		mutable.CharacterHitDiceSchema(),
		mutable.CharacterDeathSavesSchema(),
		mutable.CharacterRelationshipsSchema(),
		mutable.CharacterFactionStandingSchema(),
		mutable.CharacterQuestLogSchema(),
		mutable.SessionLogSchema(),
		mutable.CharacterSessionStatsSchema(),
		mutable.CharacterCharSheetSchema(),
		mutable.OOCPlayerSchema(),
		mutable.CharacterPendingLevelUpSchema(),
		mutable.CharacterLevelUpDecisionsSchema(),

		// Base Reference Data (Immutable)
		immutable.ImmutableClassSchema(),
		immutable.ImmutableSubClassSchema(),
		immutable.ImmutableClassFeatureSchema(),
		immutable.ImmutableSubClassFeatureSchema(),

		// race reference data
		immutable.ImmutableRaceSchema(),
		immutable.ImmutableRaceFeatureSchema(),

		// feat reference data
		immutable.ImmutableFeatSchema(),
		immutable.ImmutableFeatFeaturesSchema(),
		immutable.ImmutableBoonSchema(),
		immutable.ImmutableBoonFeatureSchema(),

		// eldritch invocation reference data
		immutable.ImmutableEldritchInvocationSchema(),

		// fighting style reference data
		immutable.ImmutableFightingStyleSchema(),

		// condition reference data
		immutable.ImmutableConditionSchema(),
		immutable.ImmutableConditionEffectSchema(),

		// weapon data
		immutable.ImmutableWeaponSchema(),
		immutable.ImmutableWeaponMasterySchema(),

		// background data
		immutable.ImmutableBackgroundSchema(),
		immutable.ImmutableBackgroundFeatureSchema(),

		// spell reference data
		immutable.ImmutableSpellSchema(),
		immutable.ImmutableSpellComponentSchema(),

		// monster reference data
		immutable.ImmutableMonsterSchema(),
		immutable.ImmutableMonsterActionSchema(),
		immutable.ImmutableMonsterTraitSchema(),
		immutable.ImmutableMonsterLegendarySchema(),
		immutable.ImmutableMonsterSpellcastingSchema(),
		immutable.ImmutableMonsterSpellListSchema(),

		// proficiency data
		immutable.ImmutableProficiencySchema(),

		// bastion data
		immutable.ImmutableConnectionLevelSchema(),
		immutable.ImmutableStrongholdTierSchema(),
		immutable.ImmutableFacilityTypeSchema(),
		immutable.ImmutableFacilityBenefitSchema(),
		immutable.ImmutableFacilityUpgradeSchema(),
		immutable.ImmutableFacilityDiscountSchema(),

		// subraces must come after races
		immutable.ImmutableSubraceSchema(),
		immutable.ImmutableSubraceFeatureSchema(),

		// armor reference data
		immutable.ImmutableArmorSchema(),

		// language reference data
		immutable.ImmutableLanguageSchema(),

		// item reference data
		immutable.ImmutableMundaneItemSchema(),
		immutable.ImmutableMagicItemSchema(),
		immutable.ImmutableMagicItemAbilitySchema(),
		immutable.ImmutablePotionSchema(),
		immutable.ImmutableItemInteractionSchema(),

		// multiclass spell slot reference data
		immutable.ImmutableClassSpellcastingProgressionSchema(),
		immutable.ImmutableMulticlassSpellSlotSchema(),
		immutable.ImmutablePactMagicSlotSchema(),
		immutable.ImmutableSingleClassSpellSlotSchema(),
	}
	for _, q := range schemas {
		if _, err := DB.Exec(q); err != nil {
			log.Fatal("Failed to create table: ", err)
		}
	}
}
