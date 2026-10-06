package descriptor

var Contract = Object(Fields{
	"provides": Object(Fields{
		"rest": Map(Endpoint, methods),
	}),
	"consumes": Map(ServiceName, Object(Fields{
		"rest": Map(Endpoint, methods),
	})),
	"schemas": Map(SchemaName, Schema()),
})

var methods = Object(Fields{
	"get":    Object(Fields{"responses": responses}),
	"post":   Object(Fields{"request": SchemaRef, "responses": responses}),
	"put":    Object(Fields{"request": SchemaRef, "responses": responses}),
	"delete": Object(Fields{"responses": responses}),
})

var responses = Map(StatusCode, SchemaRef)
