package mutable

// ChannelConfigSchema — simplified to the columns infoGatherer and the channel
// classifier actually use. respond_chance/cooldown_secs etc. were not referenced
// anywhere in the bot code and caused a schema description mismatch.
func ChannelConfigSchema() string {
	return `CREATE TABLE IF NOT EXISTS channel_config (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		channel_id   TEXT UNIQUE NOT NULL,
		mode         TEXT,
		character_id INTEGER REFERENCES characters(id)
	)`
}
