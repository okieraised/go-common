package mqtt_client

import (
	"crypto/tls"
	"fmt"
	"net/url"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/okieraised/go-common/config"
	"github.com/spf13/viper"
)

func getBool(key string, def bool) bool {
	if !viper.IsSet(key) {
		return def
	}
	return viper.GetBool(key)
}

// readDuration accepts either a duration string ("10s", "500ms")
// or an integer representing seconds; returns def if unset/zero.
func readDuration(key string, def time.Duration) time.Duration {
	if !viper.IsSet(key) {
		return def
	}
	// prefer the duration parser if a string is provided
	if s := viper.GetString(key); s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			return d
		}
	}
	// fall back to integer seconds (compat)
	if n := viper.GetInt(key); n > 0 {
		return time.Duration(n) * time.Second
	}
	// fall back to native duration (if user already set via viper.Set)
	if d := viper.GetDuration(key); d > 0 {
		return d
	}
	return def
}

func isSecureScheme(u string) bool {
	s := strings.ToLower(u)
	return strings.HasPrefix(s, "mqtts://") || strings.HasPrefix(s, "ssl://") || strings.HasPrefix(s, "tls://") || strings.HasPrefix(s, "wss://")
}

var defaultPublishHandler mqtt.MessageHandler = func(_ mqtt.Client, msg mqtt.Message) {
}

var defaultConnLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
}

var defaultConnAttemptHandler mqtt.ConnectionAttemptHandler = func(_ *url.URL, _ *tls.Config) *tls.Config {
	// return nil to keep existing TLS config; override in options if needed
	return nil
}

var defaultReconnectHandler mqtt.ReconnectHandler = func(_ mqtt.Client, _ *mqtt.ClientOptions) {
}

// NewMQTTClient creates and connects an MQTT client using viper-powered config.
// Required: uri, clientID
func NewMQTTClient(uri, clientID string) (mqtt.Client, error) {

	// Booleans
	cleanSession := getBool(config.MqttCleanSession, true)
	autoReconnect := getBool(config.MqttAutoReconnect, true)
	connectRetry := getBool(config.MqttConnectRetry, true)
	resumeSubs := getBool(config.MqttResumeSubs, true)

	// Durations (with sane defaults + multiple formats)
	writeTimeout := readDuration(config.MqttWriteTimeout, constants.DefaultMqttWriteTimeout)
	keepAlive := readDuration(config.MqttKeepAliveDuration, constants.DefaultMqttKeepAlive)
	pingTimeout := readDuration(config.MqttPingTimeout, constants.DefaultMqttPingTimeout)
	maxReconnectInterval := readDuration(config.MqttMaxConnectInterval, constants.DefaultMqttMaxReconnectInterval)
	connectTimeout := readDuration(config.MqttConnectTimeout, constants.DefaultMqttConnectTimeout)

	opts := mqtt.NewClientOptions().
		AddBroker(uri).
		SetClientID(clientID).
		SetDefaultPublishHandler(defaultPublishHandler).
		SetConnectionLostHandler(defaultConnLostHandler).
		SetReconnectingHandler(defaultReconnectHandler).
		SetConnectionAttemptHandler(defaultConnAttemptHandler).
		SetCleanSession(cleanSession).
		SetAutoReconnect(autoReconnect).
		SetConnectRetry(connectRetry).
		SetConnectRetryInterval(constants.DefaultMqttReconnectWaitInterval).
		SetMaxReconnectInterval(maxReconnectInterval).
		SetWriteTimeout(writeTimeout).
		SetKeepAlive(keepAlive).
		SetPingTimeout(pingTimeout).
		SetResumeSubs(resumeSubs).
		SetConnectTimeout(connectTimeout)

	if isSecureScheme(uri) {
		insecure := getBool(config.MqttTLSInsecureSkipVerify, false)
		opts.SetTLSConfig(&tls.Config{InsecureSkipVerify: insecure}) // #nosec G402
	}

	client := mqtt.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(connectTimeout) {
		return client, fmt.Errorf("mqtt connect timeout after %s", connectTimeout)
	}
	if err := token.Error(); err != nil {
		return client, fmt.Errorf("mqtt connect error: %w", err)
	}

	return client, nil
}
