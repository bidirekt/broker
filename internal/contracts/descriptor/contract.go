package descriptor

func Contract(protocolBlocks Fields, vocabulary Vocabulary) Node {
	return Object(Fields{
		"provides": Object(protocolBlocks),
		"consumes": Map(ServiceName, Object(protocolBlocks)),
		"schemas":  Map(SchemaName, vocabulary.SchemaAt("schema")),
	})
}
