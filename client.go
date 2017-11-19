package lruchal

type ClientConfig struct {
	Address string
}

func NewDefaultClientConfig() *ClientConfig {
	c := &ClientConfig{
		Address: ":8182",
	}
}

type Client struct {
	addr string
}

func NewDefaultClient() (*Client, error) {

}
