package utils

import (
	"context"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/log"
	"fernandoglatz/openai-compatible-proxy/internal/infrastructure/config"
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var client mqtt.Client

// ConnectMQTT initializes and connects to the MQTT broker
func ConnectMQTT(ctx context.Context) error {
	mqttConfig := config.ApplicationConfig.MQTT

	if !mqttConfig.Enabled {
		log.Info(ctx).Msg("MQTT is disabled in configuration")
		return nil
	}

	log.Info(ctx).Msg("Connecting to MQTT broker: " + mqttConfig.Broker)

	opts := mqtt.NewClientOptions()
	opts.AddBroker(mqttConfig.Broker)
	opts.SetClientID(mqttConfig.ClientID)

	if mqttConfig.Username != "" {
		opts.SetUsername(mqttConfig.Username)
	}

	if mqttConfig.Password != "" {
		opts.SetPassword(mqttConfig.Password)
	}

	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second)
	opts.SetMaxReconnectInterval(1 * time.Minute)

	opts.SetConnectionLostHandler(func(c mqtt.Client, err error) {
		log.Warn(ctx).Msg(fmt.Sprintf("MQTT connection lost: %v", err))
	})

	opts.SetReconnectingHandler(func(c mqtt.Client, opts *mqtt.ClientOptions) {
		log.Info(ctx).Msg("MQTT reconnecting...")
	})

	opts.SetOnConnectHandler(func(c mqtt.Client) {
		log.Info(ctx).Msg("MQTT connected successfully")
	})

	client = mqtt.NewClient(opts)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %v", token.Error())
	}

	log.Info(ctx).Msg("MQTT client connected")
	return nil
}

// PublishMessage publishes a message to the configured MQTT topic
func PublishMessage(ctx context.Context, message string) error {
	mqttConfig := config.ApplicationConfig.MQTT

	if !mqttConfig.Enabled {
		log.Debug(ctx).Msg("MQTT is disabled, skipping message publish")
		return nil
	}

	if client == nil || !client.IsConnected() {
		return fmt.Errorf("MQTT client is not connected")
	}

	log.Info(ctx).Msg(fmt.Sprintf("Publishing MQTT message to topic '%s': %s", mqttConfig.Topic, message))

	token := client.Publish(mqttConfig.Topic, mqttConfig.QOS, false, message)
	token.Wait()

	if token.Error() != nil {
		return fmt.Errorf("failed to publish MQTT message: %v", token.Error())
	}

	log.Info(ctx).Msg("MQTT message published successfully")
	return nil
}

// Disconnect closes the MQTT connection
func Disconnect(ctx context.Context) {
	if client != nil && client.IsConnected() {
		log.Info(ctx).Msg("Disconnecting MQTT client")
		client.Disconnect(250)
	}
}

// IsConnected returns true if the MQTT client is connected
func IsConnected() bool {
	return client != nil && client.IsConnected()
}
