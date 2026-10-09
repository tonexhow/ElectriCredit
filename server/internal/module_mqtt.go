package internal

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// moduleMQTTBridge uses a local Mosquitto installation when available. This
// keeps the Go server dependency-light while giving ESP32 modules MQTT's
// reconnect/QoS-friendly work-mode transport. If Mosquitto is unavailable,
// authenticated WebSocket remains available with the exact same JSON events.
type moduleMQTTBridge struct {
	api      *API
	host     string
	port     string
	username string
	password string
	tls      bool
}

func newModuleMQTTBridge(api *API) *moduleMQTTBridge {
	host := strings.TrimSpace(os.Getenv("ELECTRICREDIT_MQTT_HOST"))
	if host == "" {
		host = "127.0.0.1"
	}
	port := strings.TrimSpace(os.Getenv("ELECTRICREDIT_MQTT_PORT"))
	if port == "" {
		port = "1883"
	}
	tlsEnabled := strings.EqualFold(strings.TrimSpace(os.Getenv("ELECTRICREDIT_MQTT_TLS")), "true") || strings.TrimSpace(os.Getenv("ELECTRICREDIT_MQTT_TLS")) == "1"
	return &moduleMQTTBridge{api: api, host: host, port: port, username: strings.TrimSpace(os.Getenv("ELECTRICREDIT_MQTT_MODULE_USER")), password: os.Getenv("ELECTRICREDIT_MQTT_MODULE_PASSWORD"), tls: tlsEnabled}
}

func (b *moduleMQTTBridge) enabled() bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("ELECTRICREDIT_MQTT_ENABLED")))
	if raw == "0" || raw == "false" || raw == "no" {
		return false
	}
	_, subErr := exec.LookPath("mosquitto_sub")
	_, pubErr := exec.LookPath("mosquitto_pub")
	return subErr == nil && pubErr == nil
}

func (b *moduleMQTTBridge) commonArgs() []string {
	args := []string{"-h", b.host, "-p", b.port}
	if b.username != "" {
		args = append(args, "-u", b.username)
	}
	if b.password != "" {
		args = append(args, "-P", b.password)
	}
	if b.tls {
		ca := strings.TrimSpace(os.Getenv("ELECTRICREDIT_MQTT_CA"))
		if ca != "" {
			args = append(args, "--cafile", ca)
		}
	}
	return args
}

func (b *moduleMQTTBridge) start(ctx context.Context) {
	if !b.enabled() {
		return
	}
	go b.run(ctx)
}

func (b *moduleMQTTBridge) run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		args := append(b.commonArgs(), "-q", "1", "-v", "-t", "electricredit/+/+/event")
		cmd := exec.CommandContext(ctx, "mosquitto_sub", args...)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		cmd.Stderr = os.Stderr
		if err = cmd.Start(); err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			topic, payload, ok := strings.Cut(line, " ")
			if !ok {
				continue
			}
			b.handle(topic, []byte(payload))
		}
		_ = cmd.Wait()
		if ctx.Err() != nil {
			return
		}
		log.Printf("ElectriCredit MQTT listener reconnecting...")
		time.Sleep(2 * time.Second)
	}
}

func (b *moduleMQTTBridge) handle(topic string, raw []byte) {
	parts := strings.Split(strings.TrimSpace(topic), "/")
	if len(parts) != 4 || parts[0] != "electricredit" || parts[3] != "event" {
		return
	}
	kind := strings.ToLower(parts[1])
	id, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return
	}
	var msg moduleRuntimeMessage
	if json.Unmarshal(raw, &msg) != nil {
		return
	}
	response := b.api.handleModuleRuntimeMessage(kind, id, msg)
	response["transport"] = "mqtt"
	b.publish(fmt.Sprintf("electricredit/%s/%d/command", kind, id), response)
}

func (b *moduleMQTTBridge) publish(topic string, payload any) bool {
	raw, err := json.Marshal(payload)
	if err != nil {
		return false
	}
	args := append(b.commonArgs(), "-q", "1", "-t", topic, "-m", string(raw))
	cmd := exec.Command("mosquitto_pub", args...)
	return cmd.Run() == nil
}

func moduleMQTTBootstrap(serverURL, kind string, id int64, token string) map[string]any {
	enabled := true
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("ELECTRICREDIT_MQTT_ENABLED")))
	if raw == "0" || raw == "false" || raw == "no" {
		enabled = false
	}
	host := strings.TrimSpace(os.Getenv("ELECTRICREDIT_MQTT_PUBLIC_HOST"))
	if host == "" {
		if parsed, err := url.Parse(serverURL); err == nil {
			host = parsed.Hostname()
		}
	}
	port := strings.TrimSpace(os.Getenv("ELECTRICREDIT_MQTT_PORT"))
	if port == "" {
		port = "1883"
	}
	return map[string]any{
		"enabled":       enabled,
		"host":          host,
		"port":          port,
		"tls":           strings.EqualFold(strings.TrimSpace(os.Getenv("ELECTRICREDIT_MQTT_TLS")), "true") || strings.TrimSpace(os.Getenv("ELECTRICREDIT_MQTT_TLS")) == "1",
		"username":      strings.TrimSpace(os.Getenv("ELECTRICREDIT_MQTT_MODULE_USER")),
		"password":      os.Getenv("ELECTRICREDIT_MQTT_MODULE_PASSWORD"),
		"event_topic":   fmt.Sprintf("electricredit/%s/%d/event", kind, id),
		"command_topic": fmt.Sprintf("electricredit/%s/%d/command", kind, id),
		"module_token":  token,
		"qos":           1,
	}
}
