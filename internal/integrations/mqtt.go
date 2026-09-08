package integrations

import (
	"crypto/tls"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"os"
	"time"
)

type Publisher struct{ Client mqtt.Client }

func NewMQTT() (*Publisher, error) {
	broker := os.Getenv("MQTT_BROKER")
	if broker == "" {
		return nil, fmt.Errorf("MQTT_BROKER is required")
	}
	opts := mqtt.NewClientOptions().AddBroker(broker).SetClientID(os.Getenv("MQTT_CLIENT_ID"))
	opts.SetUsername(os.Getenv("MQTT_USERNAME"))
	opts.SetPassword(os.Getenv("MQTT_PASSWORD"))
	opts.SetKeepAlive(30 * time.Second)
	if os.Getenv("MQTT_TLS") == "true" {
		opts.SetTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12, ServerName: os.Getenv("MQTT_SERVER_NAME")})
	}
	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}
	return &Publisher{Client: c}, nil
}
func (p *Publisher) Publish(topic string, payload []byte) error {
	t := p.Client.Publish(topic, 1, false, payload)
	t.Wait()
	return t.Error()
}
func (p *Publisher) Close() {
	if p != nil && p.Client.IsConnected() {
		p.Client.Disconnect(1000)
	}
}
