package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Setting holds the schema definition for the Setting entity.
// It stores CLI configuration as key-value pairs.
type Setting struct {
	ent.Schema
}

// Fields of the Setting.
func (Setting) Fields() []ent.Field {
	return []ent.Field{
		field.String("key").
			NotEmpty().
			Comment("Setting name"),
		field.String("value").
			Comment("Setting value"),
	}
}

// Indexes of the Setting.
func (Setting) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("key").Unique(),
	}
}

// Edges of the Setting.
func (Setting) Edges() []ent.Edge {
	return []ent.Edge{}
}

// Annotations of the Setting.
func (Setting) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "settings"},
	}
}
