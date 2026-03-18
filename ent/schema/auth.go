package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// Auth holds the schema definition for the Auth entity.
// It uses a single-row pattern (id=1) to store the current authentication state.
type Auth struct {
	ent.Schema
}

// Fields of the Auth.
func (Auth) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").
			Default(1).
			Immutable().
			Comment("Fixed ID for single-row pattern"),
		field.String("access_token").
			Optional().
			Sensitive().
			Comment("OAuth access token"),
		field.String("refresh_token").
			Optional().
			Sensitive().
			Comment("OAuth refresh token"),
		field.Int64("expires_at").
			Optional().
			Nillable().
			Comment("Token expiry time in milliseconds since epoch"),
		field.Int64("issued_at").
			Optional().
			Nillable().
			Comment("Token issue time in milliseconds since epoch"),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Comment("When this record was last updated"),
	}
}

// Edges of the Auth.
func (Auth) Edges() []ent.Edge {
	return []ent.Edge{}
}

// Annotations of the Auth.
func (Auth) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "auth"},
	}
}
