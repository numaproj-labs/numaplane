package kubernetes

type Client interface {
	ApplyResource([]byte) error
}
