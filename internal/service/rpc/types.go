package rpc

type BackendHandlerService[T any] interface {
	Start() error
	Close() error
	HandleBackend() (T, error)
	IsAvailable() bool
}
