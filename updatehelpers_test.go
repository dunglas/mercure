package mercure

// testUpdate builds an update from a canonical-plus-alternates topic list.
func testUpdate(u *Update, topics ...string) *Update {
	u.setTopics(topics)

	return u
}
