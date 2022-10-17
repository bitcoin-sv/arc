package models

// BaseModels definitions for auto migrations
var BaseModels = []interface{}{
	// Fee database model
	&Fee{
		Model: *NewBaseModel(ModelNameFee),
	},
}
