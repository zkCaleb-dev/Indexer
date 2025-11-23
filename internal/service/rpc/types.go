package rpc

// BackendHandlerService defines the generic interface for managing backend lifecycle and access
type BackendHandlerService[T any] interface {
	Start() error                 // Initialize the backend
	Close() error                 // Shutdown the backend
	HandleBackend() (T, error)    // Retrieve the backend instance
	IsAvailable() bool            // Check if the backend is ready
}
