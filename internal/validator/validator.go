package validator

type FeeValidation byte

const (
	NoneFeeValidation FeeValidation = iota
	StandardFeeValidation
	CumulativeFeeValidation
)

const DustLimit = 1

type ScriptValidation byte

const (
	NoneScriptValidation ScriptValidation = iota
	StandardScriptValidation
)
