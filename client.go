package lruchal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"runtime"
	"time"
)

type ClientConfig struct {
	Address    string
	HttpClient *http.Client
}

func NewDefaultClientConfig() *ClientConfig {
	c := &ClientConfig{
		Address: fmt.Sprintf("127.0.0.1:%d", DefaultPort),
	}

	c.HttpClient = &http.Client{
		// pulled from https://github.com/hashicorp/go-cleanhttp/blob/master/cleanhttp.go#L23 because hashi is awesome.
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			MaxIdleConnsPerHost:   runtime.GOMAXPROCS(0) + 1,
		},
	}

	return c
}

type Client struct {
	addr   string
	client *http.Client
}

func NewDefaultClient() (*Client, error) {
	return newClient(NewDefaultClientConfig())
}

func NewClient(config *ClientConfig) (*Client, error) {
	return newClient(config)
}

func newClient(config *ClientConfig) (*Client, error) {
	def := NewDefaultClientConfig()
	if config.Address != "" {
		def.Address = config.Address
	}
	if config.HttpClient != nil {
		// TODO: replicate
		def.HttpClient = config.HttpClient
	}

	c := &Client{
		addr:   def.Address,
		client: def.HttpClient,
	}

	return c, nil
}

func (c *Client) Get(key string) (interface{}, error) {
	resp, err := c.client.Get(fmt.Sprintf("http://%s/get/%s", c.addr, key))
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 200 {
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("unable to read response: %s", err)
		}
		var data interface{}
		err = json.Unmarshal(b, &data)
		if err != nil {
			return nil, fmt.Errorf("unable to unmarshal data: %s", err)
		}
		return data, nil
	}

	return nil, fmt.Errorf("%d: %s", resp.StatusCode, resp.Status)
}

func (c *Client) Put(item Item) error {
	b, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("unable to serialize: %s", err)
	}

	req, err := http.NewRequest("PUT", fmt.Sprintf("http://%s/put", c.addr), bytes.NewBuffer(b))
	if err != nil {
		return fmt.Errorf("unable to create request: %s", err)
	}

	resp, err := c.client.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return err
	}

	if resp.StatusCode == 204 {
		return nil
	}

	b, _ = ioutil.ReadAll(resp.Body)
	return fmt.Errorf("%d: %s", resp.StatusCode, string(b))
}
