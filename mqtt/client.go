package mqtt

import (
	"encoding/json"
	"fmt"
	"log"

	paho "github.com/eclipse/paho.mqtt.golang"
)

type Client struct {
	topicRoot string
	opts      *paho.ClientOptions
	client    paho.Client
	tokens    []paho.Token
}

func NewClient(brokerURL string, clientID string, topicRoot string) *Client {
	opts := paho.NewClientOptions().AddBroker(brokerURL)
	opts.SetClientID(clientID)

	return &Client{
		topicRoot: topicRoot,
		opts:      opts,
	}
}

func (c *Client) Connect() error {
	c.client = paho.NewClient(c.opts)
	token := c.client.Connect()
	token.Wait()
	err := token.Error()
	if err != nil {
		return fmt.Errorf("connect error: %w", err)
	}

	return nil
}

func (c *Client) Disconnect() {
	if c.client == nil {
		return
	}

	c.client.Disconnect(250)
}

func (c *Client) Publish(topic string, payload any, retained bool) error {
	if c.client == nil {
		return fmt.Errorf("client not connected")
	}
	if len(topic) == 0 {
		return fmt.Errorf("topic is empty")
	}
	if topic[0] == '/' {
		return fmt.Errorf("expected relative topic (cannot begin with slash)")
	}

	scopedTopic := fmt.Sprintf("%s/%s", c.topicRoot, topic)

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("unable to encode payload")
	}

	token := c.client.Publish(scopedTopic, 0, retained, payloadBytes)
	go func() {
		token.Wait()
		err := token.Error()
		if err != nil {
			log.Printf("error publishing %v to %s: %s", payload, scopedTopic, err.Error())
		}
	}()

	return nil
}
