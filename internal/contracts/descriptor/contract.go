package descriptor

func Contract(provides Fields, consumes Fields, vocabulary Vocabulary) Node {
	return Object(Fields{
		"provides": Object(provides),
		"consumes": Map(ServiceName, Object(consumes)),
		"schemas":  Map(SchemaName, vocabulary.SchemaAt("schema")),
	})
}
