package models

// BaseModels definitions for auto migrations
var BaseModels = []interface{}{
	// Policy database model
	&Policy{
		Model: *NewBaseModel(ModelNamePolicy),
	},
}
