package asynccaller

type CallerClientI[T any] interface {
	Caller(data *T) error
	MarshalString(data *T) (string, error)
	UnmarshalString(data string) (*T, error)
}
