package immutable

func ImmutableFeatFeaturesSchema() string {
	return `CREATE TABLE IF NOT EXISTS feat_features (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    feat_id     INTEGER REFERENCES feats(id),
    description TEXT NOT NULL
)`
}
