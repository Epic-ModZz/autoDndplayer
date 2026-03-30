package db

import (
	"log"
)

// Seed inserts the baseline character and admin user rows if they don't
// already exist. Safe to call on every startup — all statements use
// INSERT OR IGNORE so re-running never overwrites live data.
//
// Fill in every TODO before deploying.
func Seed() {
	if DB == nil {
		log.Fatal("Seed: DB not initialized")
	}

	seedStatements := []struct {
		label string
		sql   string
		args  []interface{}
	}{
		// ----------------------------------------------------------------
		// Admin player
		// Fill in your Discord user ID, username, and display name below.
		// ----------------------------------------------------------------
		{
			label: "admin discord_user",
			sql: `INSERT OR IGNORE INTO discord_users
				(discord_user_id, username, display_name, is_dm, is_bot, joined_at, first_seen_at, last_seen_at)
				VALUES (?, ?, ?, 0, 0, datetime('now'), datetime('now'), datetime('now'))`,
			args: []interface{}{
				"TODO_YOUR_DISCORD_USER_ID",  // e.g. "123456789012345678"
				"TODO_YOUR_DISCORD_USERNAME", // e.g. "taner"
				"TODO_YOUR_DISPLAY_NAME",     // e.g. "Taner"
			},
		},

		// ----------------------------------------------------------------
		// Character: Caelan Mourne
		//
		// Half-Elf Paladin (Oath of Devotion — sliding toward Oathbreaker).
		// Presents as a devout, steadfast holy warrior. In practice has
		// begun rationalising increasingly dark choices in service of
		// personal ambition. The gap between the public face and the
		// private ledger is the core of the corruption arc.
		//
		// race_id and background_id are set to 1 as placeholders.
		// Update them to the correct IDs once the immutable tables are seeded.
		// ----------------------------------------------------------------
		{
			label: "character row",
			sql: `INSERT OR IGNORE INTO characters
				(id, name, race_id, background_id, alignment, experience, level,
				 hp, max_hp, temp_hp, armor_class, speed, proficiency_bonus, discord_user_id)
				VALUES (1, 'Caelan Mourne', 1, 1, 'Lawful Neutral', 0, 1, 10, 10, 0, 18, 30, 2, ?)`,
			// AC 18 = chain mail (16) + shield (2). Paladin starting equipment.
			// HP 10 = d10 hit die max at level 1.
			args: []interface{}{
				"TODO_YOUR_DISCORD_USER_ID",
			},
		},

		// ----------------------------------------------------------------
		// Personality
		// Outwardly: composed, principled, the kind of person others trust
		// with sensitive information. Inwardly: everything is a calculation.
		// ----------------------------------------------------------------
		{
			label: "character_personality",
			sql: `INSERT OR IGNORE INTO character_personality
				(character_id, traits, ideals, bonds, flaws)
				VALUES (1, ?, ?, ?, ?)`,
			args: []interface{}{
				"Calm and deliberate in speech. Rarely shows anger. Has a way of making people feel heard that they mistake for kindness.",
				"Order is a tool. Laws exist to protect those who know how to use them and punish those who don't.",
				"I swore an oath to a god who I no longer fully believe in. I keep the performance because it is useful, not because I feel it.",
				"I cannot stop collecting leverage. Even when I like someone, I note their weaknesses. I tell myself it is prudence.",
			},
		},

		// ----------------------------------------------------------------
		// Backstory
		// ----------------------------------------------------------------
		{
			label: "character_backstory",
			sql:   `INSERT OR IGNORE INTO character_backstory (character_id, backstory) VALUES (1, ?)`,
			args: []interface{}{
				`Caelan was raised in a temple orphanage after being left there as an infant — half-elven, which meant belonging fully to neither community that passed through the doors. The priests were not unkind, but the institution was transactional in ways a child eventually notices. Faith was the currency. Compliance was rewarded. Doubt was managed.

By adolescence Caelan had become genuinely devout, or believed they had. The distinction between true belief and deeply conditioned behaviour is not one most people examine closely. They were ordained as a paladin at nineteen, which was considered young but unremarkable given their aptitude.

The fracture came slowly. A mission where the "right" outcome required lying to a grieving family. A superior who buried an inconvenient miracle because it complicated a political alliance. A realisation, sitting in a cold chapel at two in the morning, that the god had not spoken to them in months and they had been filling in the silence themselves.

They did not fall dramatically. They simply stopped pretending to themselves while continuing to pretend to everyone else. The oath still holds — barely — but Caelan now maintains it as a shield and a cover, not as a conviction. The divine power keeps flowing, which they have decided not to examine too carefully.`,
			},
		},

		// ----------------------------------------------------------------
		// Public persona — what other characters see
		// ----------------------------------------------------------------
		{
			label: "character_public_persona",
			sql:   `INSERT OR IGNORE INTO character_public_persona (character_id, persona) VALUES (1, ?)`,
			args: []interface{}{
				"A composed, quietly devout paladin who seems genuinely interested in other people's problems. Trustworthy. A little reserved. The sort of person you'd feel comfortable confiding in.",
			},
		},

		// ----------------------------------------------------------------
		// Corruption arc — starting stage
		// ----------------------------------------------------------------
		{
			label: "character_corruption_arc",
			sql: `INSERT OR IGNORE INTO character_corruption_arc
				(character_id, stage, notes, triggered_at)
				VALUES (1, 'Compromised', ?, datetime('now'))`,
			args: []interface{}{
				"The internal break has happened — Caelan no longer believes but maintains the performance. Has begun making choices that would horrify their former self, always with a justification ready. Not yet visibly corrupt to other characters. The facade is intact.",
			},
		},

		// ----------------------------------------------------------------
		// Goals
		// ----------------------------------------------------------------
		{
			label: "character_goal_1",
			sql: `INSERT OR IGNORE INTO character_goals (character_id, goal, priority, status)
				VALUES (1, ?, 3, 'active')`,
			args: []interface{}{
				"Gain a position of genuine institutional authority — a seat on a guild council, a church advisory role, or equivalent. Not for the title but for the access and the legitimacy it provides.",
			},
		},
		{
			label: "character_goal_2",
			sql: `INSERT OR IGNORE INTO character_goals (character_id, goal, priority, status)
				VALUES (1, ?, 3, 'active')`,
			args: []interface{}{
				"Identify which other adventurers in the westmarch have secrets worth knowing. Not to use immediately — to hold.",
			},
		},
		{
			label: "character_goal_3",
			sql: `INSERT OR IGNORE INTO character_goals (character_id, goal, priority, status)
				VALUES (1, ?, 2, 'active')`,
			args: []interface{}{
				"Locate a source of power that does not require maintaining the current theological pretense. Warlock pact, relic, or ancient compact — the form is less important than the independence.",
			},
		},

		// ----------------------------------------------------------------
		// Active schemes
		// ----------------------------------------------------------------
		{
			label: "character_scheme_1",
			sql: `INSERT OR IGNORE INTO character_schemes (character_id, scheme, status, notes)
				VALUES (1, ?, 'active', ?)`,
			args: []interface{}{
				"Cultivate trust with the other westmarch adventurers by being reliably useful and never visibly wanting anything in return.",
				"People who feel indebted are predictable. People who trust you tell you things. Neither relationship requires genuine friendship to maintain.",
			},
		},
		{
			label: "character_scheme_2",
			sql: `INSERT OR IGNORE INTO character_schemes (character_id, scheme, status, notes)
				VALUES (1, ?, 'active', ?)`,
			args: []interface{}{
				"Maintain the paladin performance at all costs while quietly probing the boundaries of the oath.",
				"Divine power continues to flow. The god either doesn't notice the internal state or doesn't care. Either answer is useful.",
			},
		},
	}

	for _, s := range seedStatements {
		if _, err := DB.Exec(s.sql, s.args...); err != nil {
			// Log loudly but don't fatal — a bad seed row shouldn't kill the bot.
			log.Printf("Seed warning [%s]: %v", s.label, err)
		} else {
			log.Printf("Seed OK: %s", s.label)
		}
	}
}
