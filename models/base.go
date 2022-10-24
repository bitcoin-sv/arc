package models

// BaseModels definitions for auto migrations
var BaseModels = []interface{}{
	// Policy database model
	&LogAccess{
		Model: *NewBaseModel(ModelNameLogAccess),
	},
	&LogError{
		Model: *NewBaseModel(ModelNameLogError),
	},
	&Policy{
		Model: *NewBaseModel(ModelNamePolicy),
	},
	&Transaction{
		Model: *NewBaseModel(ModelNamePolicy),
	},
}
