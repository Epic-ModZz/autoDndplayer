package bot

import (
	dbpkg "PCL/db/SQL_CharStats"
	"fmt"
	"log"
	"strings"
)

// SchemaBatch groups related table definitions together for a single LLM call.
type SchemaBatch struct {
	Name   string
	Tables string
}

// schemaBatches defines the semantic groupings of your tables.
// Column lists are derived directly from the seeder INSERT statements and
// the DB schema functions — keep in sync with any schema migrations.
var schemaBatches = []SchemaBatch{
	// -------------------------------------------------------------------------
	// MUTABLE — Character data that changes during play
	// -------------------------------------------------------------------------
	{
		Name: "Character Identity & Roleplay",
		Tables: `
			characters (id, name, race_id, background_id, alignment, experience, level, hp, max_hp, temp_hp, armor_class, speed, proficiency_bonus, discord_user_id)
			character_personality (id, character_id, traits, ideals, bonds, flaws)
			character_backstory (id, character_id, backstory)
			character_public_persona (id, character_id, persona)
			character_lies (id, character_id, lie, target, revealed)
			character_corruption_arc (id, character_id, stage, notes, triggered_at)
			character_goals (id, character_id, goal, priority, status)
			character_schemes (id, character_id, scheme, status, notes)
			character_notes (id, character_id, note, knowledge_source, created_at)
			  -- knowledge_source: 'ic' | 'ooc' | 'dm'
		`,
	},
	{
		Name: "Player Information",
		Tables: `
			discord_users (id, discord_user_id, username, display_name, is_dm, is_bot, timezone, joined_at, first_seen_at, last_seen_at)
			character_sheets (id, discord_user_id, character_id, sheet_type, sheet_id, created_at, updated_at)
		`,
	},
	{
		Name: "Character Narrative Mechanics",
		Tables: `
			character_trigger_events (id, character_id, trigger, response, priority, fired, fired_session, last_fired)
			character_agents (id, character_id, agent_name, loyalty, what_they_know, role, status, location, notes)
			  -- status: 'active' | 'dead' | 'captured' | 'turned'
			channel_config (id, channel_id, mode, character_id)
		`,
	},
	{
		Name: "Character Combat State",
		Tables: `
			character_conditions (id, character_id, condition_name, condition_id, duration, source)
			character_exhaustion (id, character_id, level)
			character_concentration (id, character_id, spell_id, spell_name, cast_at_level, duration_remaining, started_at)
			character_death_saves (id, character_id, successes, failures, stable)
			character_hit_dice (id, character_id, class_id, die_type, total, remaining, maximum)
			character_inspiration (id, character_id, has_inspiration)
		`,
	},
	{
		Name: "Character Resources & Spell Slots",
		Tables: `
			character_classes (id, character_id, class_id, subclass_id, level, is_primary)
			character_class_resources (id, character_id, class_id, resource_name, total, remaining, resets_on, current, maximum, recharge_on)
			  -- current/maximum are aliases for remaining/total; recharge_on alias for resets_on
			character_spell_slots (id, character_id, slot_level, total, remaining, maximum)
			  -- maximum is alias for total
			character_spells_known (id, character_id, spell_id, is_prepared, prepared, always_prepared)
		`,
	},
	{
		Name: "Character Abilities & Proficiencies",
		Tables: `
			character_features (id, character_id, feature_id, source, source_level)
			character_feats (id, character_id, feat_id, gained_at_level)
			character_proficiencies (id, character_id, proficiency_id, proficiency_type, type, expertise)
		`,
	},
	{
		Name: "Character Inventory & Economy",
		Tables: `
			character_inventory (id, character_id, item_name, item_type, quantity, is_equipped, equipped, is_attuned, attunement, notes)
			  -- equipped is alias for is_equipped; attunement is alias for is_attuned
			character_currency (id, character_id, pp, gp, ep, sp, cp)
		`,
	},
	{
		Name: "Character World Standing",
		Tables: `
			character_relationships (id, character_id, related_to_id, related_to_name, npc_id, relationship_type, disposition, trust_level, notes)
			  -- npc_id alias for related_to_id; relationship_type alias for disposition
			character_faction_standing (id, character_id, faction_name, reputation, standing, rank, notes)
			  -- standing is alias for reputation
			character_quest_log (id, character_id, quest_name, status, session_acquired, giver, notes, updated_at)
		`,
	},
	{
		Name: "NPCs & Session Logs",
		Tables: `
			npc_details (id, character_id, name, race, role, location, availability, disposition, alive, dialogue_style, schedule, notes, discovered_ic)
			  -- availability: 'alive' | 'dead' | 'missing' | 'imprisoned'; alive is alias for availability
			npc_secrets (id, character_id, npc_id, secret, known_by, revealed, revealed_to, discovered_ic)
			  -- npc_id alias for character_id; known_by alias for revealed_to
			session_log (id, session_number, played_at, session_date, summary, participants, dm_notes)
			  -- session_date alias for played_at
			character_session_stats (id, character_id, session_id, damage_dealt, damage_taken, kills, spells_cast, healing_done, knocks, deaths, notes)
		`,
	},

	// -------------------------------------------------------------------------
	// IMMUTABLE — Reference data that does not change
	// -------------------------------------------------------------------------
	{
		Name: "Class & Subclass Reference",
		Tables: `
			classes (id, name, hit_die, primary_ability, saving_throws, spellcasting_ability, armor_proficiencies, weapon_proficiencies, skill_choices, skill_options)
			subclasses (id, class_id, name, description, source)
			class_features (id, class_id, name, level, description)
			subclass_features (id, subclass_id, name, level, description)
			class_spellcasting_progression (id, class_id, progression_type, slot_multiplier, uses_pact_magic, preparation_type, spellcasting_ability, level, cantrips_known, spells_known, slot_level_1, slot_level_2, slot_level_3, slot_level_4, slot_level_5, slot_level_6, slot_level_7, slot_level_8, slot_level_9)
			single_class_spell_slots (id, class_id, level, slot_level_1, slot_level_2, slot_level_3, slot_level_4, slot_level_5, slot_level_6, slot_level_7, slot_level_8, slot_level_9)
		`,
	},
	{
		Name: "Race & Subrace Reference",
		Tables: `
			races (id, name, size, speed, ability_score_increases, languages, traits_summary, source)
			race_features (id, race_id, name, description, level)
			subraces (id, race_id, name, description, source)
			subrace_features (id, subrace_id, name, description, level)
		`,
	},
	{
		Name: "Feats, Boons & Invocations",
		Tables: `
			feats (id, name, prerequisite, description, source)
			feat_features (id, feat_id, description)
			boons (id, name, prerequisite, description, source)
			boon_features (id, boon_id, description)
			eldritch_invocations (id, name, prerequisite_level, prerequisite_spell, description)
			fighting_styles (id, name, description, available_to, source)
		`,
	},
	{
		Name: "Backgrounds Reference",
		Tables: `
			backgrounds (id, name, description, skill_proficiencies, tool_proficiencies, languages, equipment, source)
			background_features (id, background_id, name, description)
		`,
	},
	{
		Name: "Conditions Reference",
		Tables: `
			conditions (id, name, source, stackable, max_stacks, description)
			condition_effects (id, condition_id, stack_level, description)
		`,
	},
	{
		Name: "Spells Reference",
		Tables: `
			spells (id, name, level, school, casting_time, range, duration, concentration, ritual, description, higher_levels, classes, source)
			spell_components (id, spell_id, type, description, gold_cost, consumed)
			multiclass_spell_slots (id, total_caster_level, spellcaster_level, slot_level_1, slot_level_2, slot_level_3, slot_level_4, slot_level_5, slot_level_6, slot_level_7, slot_level_8, slot_level_9)
			pact_magic_slots (id, warlock_level, slot_level, slot_count, slots, recharge_on)
			  -- slots is alias for slot_count
		`,
	},
	{
		Name: "Weapons & Armor Reference",
		Tables: `
			weapons (id, name, weapon_type, damage_dice, damage_type, weight, cost, properties, mastery, range_normal, range_long, source)
			weapon_masteries (id, name, description, applicable_weapons, source)
			armor (id, name, armor_type, base_ac, dex_bonus, strength_required, strength_requirement, stealth_penalty, stealth_disadvantage, weight, cost_gp, cost, source)
			  -- strength_requirement alias for strength_required; stealth_disadvantage alias for stealth_penalty; cost alias for cost_gp
		`,
	},
	{
		Name: "Items Reference",
		Tables: `
			mundane_items (id, name, category, subcategory, weight, cost_gp, description, capacity, charges, notes, source)
			magic_items (id, name, rarity, item_type, subtype, requires_attunement, attunement_prereq, charges, recharge, description, grants_ac_bonus, grants_attack_bonus, grants_damage_bonus, cursed, source)
			magic_item_abilities (id, magic_item_id, name, action_type, charges_cost, spell_id, save_dc, save_ability, description)
			potions (id, magic_item_id, healing_dice, duration, description)
			item_interactions (id, mundane_item_id, magic_item_id, action_name, action_type, dc, ability, skill, tool_required, prerequisite, description)
			proficiencies (id, name, proficiency_type, ability, alternate_abilities, description)
			languages (id, name, language_type, typical_speakers, script, description, source)
		`,
	},
	{
		Name: "Monsters Reference",
		Tables: `
			monsters (id, name, size, creature_type, type, alignment, ac, hp_average, hp, speed, challenge_rating, cr, xp, proficiency_bonus, strength, str, dexterity, dex, constitution, con, intelligence, int, wisdom, wis, charisma, cha, source)
			  -- type alias for creature_type; hp/str/dex/con/int/wis/cha/cr are aliases for the full column names
			monster_actions (id, monster_id, name, action_type, attack_bonus, hit_dice, damage_type, dc_value, dc_ability, description)
			  -- action_type: 'action' | 'bonus_action' | 'reaction' | 'legendary'
			monster_traits (id, monster_id, name, description)
			monster_legendary (id, monster_id, name, cost, description)
			monster_legendary_info (id, monster_id, legendary_action_count, lair_action_initiative, lair_description)
			monster_spellcasting (id, monster_id, spellcasting_ability, ability, spell_save_dc, save_dc, spell_attack_bonus, attack_bonus, innate, notes)
			monster_spell_list (id, monster_id, spell_id, use_type, uses_per_day)
			monster_spells (id, monster_id, spell_id, uses)
		`,
	},
	{
		Name: "Bastion & Stronghold Reference",
		Tables: `
			connection_levels (id, name, rank, requirement)
			stronghold_tiers (id, facility_points, cost_gp, build_time_days, level_requirement, connection_required_id, land_requirement)
			facility_types (id, name, level_requirement, base_facility_points, cost_gp, build_time_days, prerequisite_feature, prerequisite_class, description)
			facility_benefits (id, facility_type_id, name, description, frequency, cost_gp, requires_rp)
			facility_upgrades (id, facility_type_id, upgrade_name, level_requirement, additional_fp, cost_gp, build_time_days, description)
			facility_discounts (id, facility_type_id, requires_facility_id, fp_discount)
		`,
	},
}

// gatherOOCCharacterContext runs a fixed set of direct SQL queries to pull the
// bot character's identity, class, subclass, race, personality, and recent
// notes. This replaces the LLM-generated SQL path for OOC channels, which was
// unreliable (MySQL syntax, missing joins, wrong character targeted).
func gatherOOCCharacterContext() (string, error) {
	if dbpkg.DB == nil {
		return "", nil
	}

	queries := []string{
		// Core identity with race and background names resolved
		`SELECT c.name, c.level, c.alignment, c.hp, c.max_hp, c.armor_class,
		        COALESCE(r.name, '') AS race,
		        COALESCE(b.name, '') AS background
		 FROM characters c
		 LEFT JOIN races r ON r.id = c.race_id
		 LEFT JOIN backgrounds b ON b.id = c.background_id
		 WHERE c.id = 1`,

		// Primary class and subclass
		`SELECT cl.name AS class, cc.level AS class_level,
		        COALESCE(sc.name, '') AS subclass
		 FROM character_classes cc
		 JOIN classes cl ON cl.id = cc.class_id
		 LEFT JOIN subclasses sc ON sc.class_id = cc.class_id AND sc.id = cc.subclass_id
		 WHERE cc.character_id = 1 AND cc.is_primary = 1
		 LIMIT 1`,

		// Personality
		`SELECT traits, ideals, bonds, flaws
		 FROM character_personality WHERE character_id = 1`,

		// Public persona
		`SELECT persona FROM character_public_persona WHERE character_id = 1`,

		// Active goals (top 3 by priority)
		`SELECT goal, priority, status FROM character_goals
		 WHERE character_id = 1 AND status != 'completed'
		 ORDER BY priority LIMIT 3`,

		// Recent OOC/DM notes (last 5)
		`SELECT note, knowledge_source, created_at FROM character_notes
		 WHERE character_id = 1
		 ORDER BY created_at DESC LIMIT 5`,

		// Known player characters as NPCs (discovered OOC)
		`SELECT name, race, role, notes FROM npc_details
		 WHERE discovered_ic = 0 LIMIT 10`,
	}

	var sb strings.Builder
	for _, q := range queries {
		rows, err := dbpkg.DB.Query(q)
		if err != nil {
			log.Printf("gatherOOCCharacterContext: query error: %v", err)
			continue
		}
		cols, _ := rows.Columns()
		sb.WriteString(strings.Join(cols, " | ") + "\n")
		for rows.Next() {
			vals := make([]interface{}, len(cols))
			ptrs := make([]interface{}, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			rows.Scan(ptrs...)
			parts := make([]string, len(cols))
			for i, v := range vals {
				if v == nil {
					parts[i] = "NULL"
				} else {
					parts[i] = fmt.Sprintf("%v", v)
				}
			}
			sb.WriteString(strings.Join(parts, " | ") + "\n")
		}
		rows.Close()
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

// GatherInfo pre-filters schema batches for relevance, then queries only the
// relevant ones to generate SQL for retrieving context from the DB.
func GatherInfo(job *MessageJob) (string, error) {
	// OOC channels use a hardcoded direct query for character identity rather
	// than asking the LLM to generate SQL. This guarantees the race/class joins
	// are always present and avoids the LLM generating MySQL syntax or missing
	// the join entirely. The full batch pipeline is overkill for OOC — all we
	// need is enough context for the response generator to answer game questions.
	if job.Mode == ChannelModeOOC {
		log.Println("GatherInfo: OOC channel — running direct character lookup")
		return gatherOOCCharacterContext()
	}

	// Build roleplay context from recent messages
	var context strings.Builder
	for i := len(job.Messages) - 1; i >= 0; i-- {
		m := job.Messages[i]
		context.WriteString(fmt.Sprintf("%s: %s\n", m.Author.Username, m.Content))
	}

	roleplay := fmt.Sprintf("%s\nLatest message from %s: \"%s\"",
		context.String(),
		job.Message.Author.Username,
		job.Message.Content,
	)

	// Pre-filter: ask the classifier model which batches are relevant.
	log.Println("GatherInfo: calling FilterRelevantBatches")
	relevantBatches, err := FilterRelevantBatches(roleplay)
	log.Printf("GatherInfo: FilterRelevantBatches done — %d batches", len(relevantBatches))
	if err != nil {
		// Non-fatal: fall back to querying all batches
		relevantBatches = schemaBatches
	}

	// Query only the relevant batches
	var allQueries strings.Builder
	config := SQLConfig()

	for _, batch := range relevantBatches {
		log.Printf("GatherInfo: querying batch '%s'", batch.Name)
		response, err := QueryWithSchema(batch.Tables, roleplay, config)
		log.Printf("GatherInfo: batch '%s' done", batch.Name)
		if err != nil {
			return "", fmt.Errorf("LLM error on batch '%s': %w", batch.Name, err)
		}

		trimmed := strings.TrimSpace(response)
		if trimmed == "" || strings.EqualFold(trimmed, "none") {
			continue
		}

		allQueries.WriteString(fmt.Sprintf("-- Batch: %s\n", batch.Name))
		allQueries.WriteString(trimmed)
		allQueries.WriteString("\n\n")
	}

	return allQueries.String(), nil
}
