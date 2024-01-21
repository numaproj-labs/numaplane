package kubernetes

type Client interface {
	ApplyResource([]byte, string) error
}
