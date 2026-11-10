package descriptor

func Contract(protocolBlocks Fields) Node {
	return Object(Fields{
		"provides": Object(protocolBlocks),
		"consumes": Map(ServiceName, Object(protocolBlocks)),
		"schemas":  Map(SchemaName, SchemaAt("schema")),
	})
}
