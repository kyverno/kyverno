/*
 * MinIO Cloud Storage, (C) 2019 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package notify

import (
	"crypto/tls"
	"crypto/x509"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio/cmd/config"
	"github.com/minio/minio/cmd/logger"
	"github.com/minio/minio/pkg/env"
	"github.com/minio/minio/pkg/event"
	"github.com/minio/minio/pkg/event/target"
	xnet "github.com/minio/minio/pkg/net"
)

const (
	formatNamespace = "namespace"
	formatAccess    = "access"
)

// TestNotificationTargets is similar to GetNotificationTargets()
// avoids explicit registration.
func TestNotificationTargets(cfg config.Config, doneCh <-chan struct{}, rootCAs *x509.CertPool) error {
	_, err := RegisterNotificationTargets(cfg, doneCh, rootCAs, true)
	return err
}

// GetNotificationTargets registers and initializes all notification
// targets, returns error if any.
func GetNotificationTargets(cfg config.Config, doneCh <-chan struct{}, rootCAs *x509.CertPool) (*event.TargetList, error) {
	return RegisterNotificationTargets(cfg, doneCh, rootCAs, false)
}

// RegisterNotificationTargets - returns TargetList which contains enabled targets in serverConfig.
// A new notification target is added like below
// * Add a new target in pkg/event/target package.
// * Add newly added target configuration to serverConfig.Notify.<TARGET_NAME>.
// * Handle the configuration in this function to create/add into TargetList.
func RegisterNotificationTargets(cfg config.Config, doneCh <-chan struct{}, rootCAs *x509.CertPool, test bool) (*event.TargetList, error) {
	targetList := event.NewTargetList()
	if err := checkValidNotificationKeys(cfg); err != nil {
		return nil, err
	}

	amqpTargets, err := GetNotifyAMQP(cfg[config.NotifyAMQPSubSys])
	if err != nil {
		return nil, err
	}

	esTargets, err := GetNotifyES(cfg[config.NotifyESSubSys])
	if err != nil {
		return nil, err
	}

	kafkaTargets, err := GetNotifyKafka(cfg[config.NotifyKafkaSubSys])
	if err != nil {
		return nil, err
	}

	mqttTargets, err := GetNotifyMQTT(cfg[config.NotifyMQTTSubSys], rootCAs)
	if err != nil {
		return nil, err
	}

	mysqlTargets, err := GetNotifyMySQL(cfg[config.NotifyMySQLSubSys])
	if err != nil {
		return nil, err
	}

	natsTargets, err := GetNotifyNATS(cfg[config.NotifyNATSSubSys])
	if err != nil {
		return nil, err
	}

	nsqTargets, err := GetNotifyNSQ(cfg[config.NotifyNSQSubSys])
	if err != nil {
		return nil, err
	}

	postgresTargets, err := GetNotifyPostgres(cfg[config.NotifyPostgresSubSys])
	if err != nil {
		return nil, err
	}

	redisTargets, err := GetNotifyRedis(cfg[config.NotifyRedisSubSys])
	if err != nil {
		return nil, err
	}

	webhookTargets, err := GetNotifyWebhook(cfg[config.NotifyWebhookSubSys], rootCAs)
	if err != nil {
		return nil, err
	}

	for id, args := range amqpTargets {
		if !args.Enable {
			continue
		}
		newTarget, err := target.NewAMQPTarget(id, args, doneCh, logger.LogOnceIf)
		if err != nil {
			return nil, err
		}
		if test {
			newTarget.Close()
			continue
		}
		if err = targetList.Add(newTarget); err != nil {
			return nil, err
		}
	}

	for id, args := range esTargets {
		if !args.Enable {
			continue
		}
		newTarget, err := target.NewElasticsearchTarget(id, args, doneCh, logger.LogOnceIf)
		if err != nil {
			return nil, err

		}
		if test {
			newTarget.Close()
			continue
		}
		if err = targetList.Add(newTarget); err != nil {
			return nil, err
		}
	}

	for id, args := range kafkaTargets {
		if !args.Enable {
			continue
		}
		args.TLS.RootCAs = rootCAs
		newTarget, err := target.NewKafkaTarget(id, args, doneCh, logger.LogOnceIf)
		if err != nil {
			return nil, err
		}
		if test {
			newTarget.Close()
			continue
		}
		if err = targetList.Add(newTarget); err != nil {
			return nil, err
		}
	}

	for id, args := range mqttTargets {
		if !args.Enable {
			continue
		}
		args.RootCAs = rootCAs
		newTarget, err := target.NewMQTTTarget(id, args, doneCh, logger.LogOnceIf)
		if err != nil {
			return nil, err
		}
		if test {
			newTarget.Close()
			continue
		}
		if err = targetList.Add(newTarget); err != nil {
			return nil, err
		}
	}

	for id, args := range mysqlTargets {
		if !args.Enable {
			continue
		}
		newTarget, err := target.NewMySQLTarget(id, args, doneCh, logger.LogOnceIf)
		if err != nil {
			return nil, err
		}
		if test {
			newTarget.Close()
			continue
		}
		if err = targetList.Add(newTarget); err != nil {
			return nil, err
		}
	}

	for id, args := range natsTargets {
		if !args.Enable {
			continue
		}
		newTarget, err := target.NewNATSTarget(id, args, doneCh, logger.LogOnceIf)
		if err != nil {
			return nil, err
		}
		if test {
			newTarget.Close()
			continue
		}
		if err = targetList.Add(newTarget); err != nil {
			return nil, err
		}
	}

	for id, args := range nsqTargets {
		if !args.Enable {
			continue
		}
		newTarget, err := target.NewNSQTarget(id, args, doneCh, logger.LogOnceIf)
		if err != nil {
			return nil, err
		}
		if test {
			newTarget.Close()
			continue
		}
		if err = targetList.Add(newTarget); err != nil {
			return nil, err
		}
	}

	for id, args := range postgresTargets {
		if !args.Enable {
			continue
		}
		newTarget, err := target.NewPostgreSQLTarget(id, args, doneCh, logger.LogOnceIf)
		if err != nil {
			return nil, err
		}
		if test {
			newTarget.Close()
			continue
		}
		if err = targetList.Add(newTarget); err != nil {
			return nil, err
		}
	}

	for id, args := range redisTargets {
		if !args.Enable {
			continue
		}
		newTarget, err := target.NewRedisTarget(id, args, doneCh, logger.LogOnceIf)
		if err != nil {
			return nil, err
		}
		if test {
			newTarget.Close()
			continue
		}
		if err = targetList.Add(newTarget); err != nil {
			return nil, err
		}
	}

	for id, args := range webhookTargets {
		if !args.Enable {
			continue
		}
		args.RootCAs = rootCAs
		newTarget, err := target.NewWebhookTarget(id, args, doneCh, logger.LogOnceIf)
		if err != nil {
			return nil, err
		}
		if test {
			newTarget.Close()
			continue
		}
		if err := targetList.Add(newTarget); err != nil {
			return nil, err
		}
	}

	return targetList, nil
}

// DefaultNotificationKVS - default notification list of kvs.
var (
	DefaultNotificationKVS = map[string]config.KVS{
		config.NotifyAMQPSubSys:     DefaultAMQPKVS,
		config.NotifyKafkaSubSys:    DefaultKafkaKVS,
		config.NotifyMQTTSubSys:     DefaultMQTTKVS,
		config.NotifyMySQLSubSys:    DefaultMySQLKVS,
		config.NotifyNATSSubSys:     DefaultNATSKVS,
		config.NotifyNSQSubSys:      DefaultNSQKVS,
		config.NotifyPostgresSubSys: DefaultPostgresKVS,
		config.NotifyRedisSubSys:    DefaultRedisKVS,
		config.NotifyWebhookSubSys:  DefaultWebhookKVS,
		config.NotifyESSubSys:       DefaultESKVS,
	}
)

func checkValidNotificationKeys(cfg config.Config) error {
	for subSys, tgt := range cfg {
		validKVS, ok := DefaultNotificationKVS[subSys]
		if !ok {
			continue
		}
		for tname, kv := range tgt {
			subSysTarget := subSys
			if tname != config.Default {
				subSysTarget = subSys + config.SubSystemSeparator + tname
			}
			if err := config.CheckValidKeys(subSysTarget, kv, validKVS); err != nil {
				return err
			}
		}
	}
	return nil
}

func mergeTargets(cfgTargets map[string]config.KVS, envname string, defaultKVS config.KVS) map[string]config.KVS {
	newCfgTargets := make(map[string]config.KVS)
	for _, e := range env.List(envname) {
		tgt := strings.TrimPrefix(e, envname+config.Default)
		if tgt == envname {
			tgt = config.Default
		}
		newCfgTargets[tgt] = defaultKVS
	}
	for tgt, kv := range cfgTargets {
		newCfgTargets[tgt] = kv
	}
	return newCfgTargets
}

// DefaultKakfaKVS - default KV for kafka target
var (
	DefaultKafkaKVS = config.KVS{
		config.State:              config.StateOff,
		config.Comment:            "Default settings for Kafka notification",
		target.KafkaTopic:         "",
		target.KafkaBrokers:       "",
		target.KafkaSASLUsername:  "",
		target.KafkaSASLPassword:  "",
		target.KafkaTLSClientAuth: "0",
		target.KafkaSASLEnable:    config.StateOff,
		target.KafkaTLSEnable:     config.StateOff,
		target.KafkaTLSSkipVerify: config.StateOff,
		target.KafkaQueueLimit:    "0",
		target.KafkaQueueDir:      "",
	}
)

// GetNotifyKafka - returns a map of registered notification 'kafka' targets
func GetNotifyKafka(kafkaKVS map[string]config.KVS) (map[string]target.KafkaArgs, error) {
	kafkaTargets := make(map[string]target.KafkaArgs)
	for k, kv := range mergeTargets(kafkaKVS, target.EnvKafkaState, DefaultKafkaKVS) {
		stateEnv := target.EnvKafkaState
		if k != config.Default {
			stateEnv = stateEnv + config.Default + k
		}
		enabled, err := config.ParseBool(env.Get(stateEnv, kv.Get(config.State)))
		if err != nil {
			return nil, err
		}
		if !enabled {
			continue
		}
		var brokers []xnet.Host
		brokersEnv := target.EnvKafkaBrokers
		if k != config.Default {
			brokersEnv = brokersEnv + config.Default + k
		}
		kafkaBrokers := env.Get(brokersEnv, kv.Get(target.KafkaBrokers))
		if len(kafkaBrokers) == 0 {
			return nil, config.Error("kafka 'brokers' cannot be empty")
		}
		for _, s := range strings.Split(kafkaBrokers, config.ValueSeparator) {
			var host *xnet.Host
			host, err = xnet.ParseHost(s)
			if err != nil {
				break
			}
			brokers = append(brokers, *host)
		}
		if err != nil {
			return nil, err
		}

		queueLimitEnv := target.EnvKafkaQueueLimit
		if k != config.Default {
			queueLimitEnv = queueLimitEnv + config.Default + k
		}
		queueLimit, err := strconv.ParseUint(env.Get(queueLimitEnv, kv.Get(target.KafkaQueueLimit)), 10, 64)
		if err != nil {
			return nil, err
		}

		clientAuthEnv := target.EnvKafkaTLSClientAuth
		if k != config.Default {
			clientAuthEnv = clientAuthEnv + config.Default + k
		}
		clientAuth, err := strconv.Atoi(env.Get(clientAuthEnv, kv.Get(target.KafkaTLSClientAuth)))
		if err != nil {
			return nil, err
		}

		topicEnv := target.EnvKafkaTopic
		if k != config.Default {
			topicEnv = topicEnv + config.Default + k
		}

		queueDirEnv := target.EnvKafkaQueueDir
		if k != config.Default {
			queueDirEnv = queueDirEnv + config.Default + k
		}

		kafkaArgs := target.KafkaArgs{
			Enable:     enabled,
			Brokers:    brokers,
			Topic:      env.Get(topicEnv, kv.Get(target.KafkaTopic)),
			QueueDir:   env.Get(queueDirEnv, kv.Get(target.KafkaQueueDir)),
			QueueLimit: queueLimit,
		}

		tlsEnableEnv := target.EnvKafkaTLSEnable
		if k != config.Default {
			tlsEnableEnv = tlsEnableEnv + config.Default + k
		}
		tlsSkipVerifyEnv := target.EnvKafkaTLSSkipVerify
		if k != config.Default {
			tlsSkipVerifyEnv = tlsSkipVerifyEnv + config.Default + k
		}
		kafkaArgs.TLS.Enable = env.Get(tlsEnableEnv, kv.Get(target.KafkaTLSEnable)) == config.StateOn
		kafkaArgs.TLS.SkipVerify = env.Get(tlsSkipVerifyEnv, kv.Get(target.KafkaTLSSkipVerify)) == config.StateOn
		kafkaArgs.TLS.ClientAuth = tls.ClientAuthType(clientAuth)

		saslEnableEnv := target.EnvKafkaSASLEnable
		if k != config.Default {
			saslEnableEnv = saslEnableEnv + config.Default + k
		}
		saslUsernameEnv := target.EnvKafkaSASLUsername
		if k != config.Default {
			saslUsernameEnv = saslUsernameEnv + config.Default + k
		}
		saslPasswordEnv := target.EnvKafkaSASLPassword
		if k != config.Default {
			saslPasswordEnv = saslPasswordEnv + config.Default + k
		}
		kafkaArgs.SASL.Enable = env.Get(saslEnableEnv, kv.Get(target.KafkaSASLEnable)) == config.StateOn
		kafkaArgs.SASL.User = env.Get(saslUsernameEnv, kv.Get(target.KafkaSASLUsername))
		kafkaArgs.SASL.Password = env.Get(saslPasswordEnv, kv.Get(target.KafkaSASLPassword))

		if err = kafkaArgs.Validate(); err != nil {
			return nil, err
		}

		kafkaTargets[k] = kafkaArgs
	}

	return kafkaTargets, nil
}

// DefaultMQTTKVS - default MQTT config
var (
	DefaultMQTTKVS = config.KVS{
		config.State:                 config.StateOff,
		config.Comment:               "Default settings for MQTT notification",
		target.MqttBroker:            "",
		target.MqttTopic:             "",
		target.MqttPassword:          "",
		target.MqttUsername:          "",
		target.MqttQoS:               "0",
		target.MqttKeepAliveInterval: "0s",
		target.MqttReconnectInterval: "0s",
		target.MqttQueueDir:          "",
		target.MqttQueueLimit:        "0",
	}
)

// GetNotifyMQTT - returns a map of registered notification 'mqtt' targets
func GetNotifyMQTT(mqttKVS map[string]config.KVS, rootCAs *x509.CertPool) (map[string]target.MQTTArgs, error) {
	mqttTargets := make(map[string]target.MQTTArgs)
	for k, kv := range mergeTargets(mqttKVS, target.EnvMQTTState, DefaultMQTTKVS) {
		stateEnv := target.EnvMQTTState
		if k != config.Default {
			stateEnv = stateEnv + config.Default + k
		}

		enabled, err := config.ParseBool(env.Get(stateEnv, kv.Get(config.State)))
		if err != nil {
			return nil, err
		}
		if !enabled {
			continue
		}

		brokerEnv := target.EnvMQTTBroker
		if k != config.Default {
			brokerEnv = brokerEnv + config.Default + k
		}
		brokerURL, err := xnet.ParseURL(env.Get(brokerEnv, kv.Get(target.MqttBroker)))
		if err != nil {
			return nil, err
		}

		reconnectIntervalEnv := target.EnvMQTTReconnectInterval
		if k != config.Default {
			reconnectIntervalEnv = reconnectIntervalEnv + config.Default + k
		}
		reconnectInterval, err := time.ParseDuration(env.Get(reconnectIntervalEnv,
			kv.Get(target.MqttReconnectInterval)))
		if err != nil {
			return nil, err
		}

		keepAliveIntervalEnv := target.EnvMQTTKeepAliveInterval
		if k != config.Default {
			keepAliveIntervalEnv = keepAliveIntervalEnv + config.Default + k
		}
		keepAliveInterval, err := time.ParseDuration(env.Get(keepAliveIntervalEnv,
			kv.Get(target.MqttKeepAliveInterval)))
		if err != nil {
			return nil, err
		}

		queueLimitEnv := target.EnvMQTTQueueLimit
		if k != config.Default {
			queueLimitEnv = queueLimitEnv + config.Default + k
		}
		queueLimit, err := strconv.ParseUint(env.Get(queueLimitEnv, kv.Get(target.MqttQueueLimit)), 10, 64)
		if err != nil {
			return nil, err
		}

		qosEnv := target.EnvMQTTQoS
		if k != config.Default {
			qosEnv = qosEnv + config.Default + k
		}

		// Parse uint8 value
		qos, err := strconv.ParseUint(env.Get(qosEnv, kv.Get(target.MqttQoS)), 10, 8)
		if err != nil {
			return nil, err
		}

		topicEnv := target.EnvMQTTTopic
		if k != config.Default {
			topicEnv = topicEnv + config.Default + k
		}

		usernameEnv := target.EnvMQTTUsername
		if k != config.Default {
			usernameEnv = usernameEnv + config.Default + k
		}

		passwordEnv := target.EnvMQTTPassword
		if k != config.Default {
			passwordEnv = passwordEnv + config.Default + k
		}

		queueDirEnv := target.EnvMQTTQueueDir
		if k != config.Default {
			queueDirEnv = queueDirEnv + config.Default + k
		}

		mqttArgs := target.MQTTArgs{
			Enable:               enabled,
			Broker:               *brokerURL,
			Topic:                env.Get(topicEnv, kv.Get(target.MqttTopic)),
			QoS:                  byte(qos),
			User:                 env.Get(usernameEnv, kv.Get(target.MqttUsername)),
			Password:             env.Get(passwordEnv, kv.Get(target.MqttPassword)),
			MaxReconnectInterval: reconnectInterval,
			KeepAlive:            keepAliveInterval,
			RootCAs:              rootCAs,
			QueueDir:             env.Get(queueDirEnv, kv.Get(target.MqttQueueDir)),
			QueueLimit:           queueLimit,
		}

		if err = mqttArgs.Validate(); err != nil {
			return nil, err
		}
		mqttTargets[k] = mqttArgs
	}
	return mqttTargets, nil
}

// DefaultMySQLKVS - default KV for MySQL
var (
	DefaultMySQLKVS = config.KVS{
		config.State:           config.StateOff,
		config.Comment:         "Default settings for MySQL notification",
		target.MySQLFormat:     formatNamespace,
		target.MySQLHost:       "",
		target.MySQLPort:       "",
		target.MySQLUsername:   "",
		target.MySQLPassword:   "",
		target.MySQLDatabase:   "",
		target.MySQLDSNString:  "",
		target.MySQLTable:      "",
		target.MySQLQueueLimit: "0",
		target.MySQLQueueDir:   "",
	}
)

// GetNotifyMySQL - returns a map of registered notification 'mysql' targets
func GetNotifyMySQL(mysqlKVS map[string]config.KVS) (map[string]target.MySQLArgs, error) {
	mysqlTargets := make(map[string]target.MySQLArgs)
	for k, kv := range mergeTargets(mysqlKVS, target.EnvMySQLState, DefaultMySQLKVS) {
		stateEnv := target.EnvMySQLState
		if k != config.Default {
			stateEnv = stateEnv + config.Default + k
		}

		enabled, err := config.ParseBool(env.Get(stateEnv, kv.Get(config.State)))
		if err != nil {
			return nil, err
		}
		if !enabled {
			continue
		}

		hostEnv := target.EnvMySQLHost
		if k != config.Default {
			hostEnv = hostEnv + config.Default + k
		}

		host, err := xnet.ParseURL(env.Get(hostEnv, kv.Get(target.MySQLHost)))
		if err != nil {
			return nil, err
		}

		queueLimitEnv := target.EnvMySQLQueueLimit
		if k != config.Default {
			queueLimitEnv = queueLimitEnv + config.Default + k
		}
		queueLimit, err := strconv.ParseUint(env.Get(queueLimitEnv, kv.Get(target.MySQLQueueLimit)), 10, 64)
		if err != nil {
			return nil, err
		}

		formatEnv := target.EnvMySQLFormat
		if k != config.Default {
			formatEnv = formatEnv + config.Default + k
		}
		dsnStringEnv := target.EnvMySQLDSNString
		if k != config.Default {
			dsnStringEnv = dsnStringEnv + config.Default + k
		}
		tableEnv := target.EnvMySQLTable
		if k != config.Default {
			tableEnv = tableEnv + config.Default + k
		}
		portEnv := target.EnvMySQLPort
		if k != config.Default {
			portEnv = portEnv + config.Default + k
		}
		usernameEnv := target.EnvMySQLUsername
		if k != config.Default {
			usernameEnv = usernameEnv + config.Default + k
		}
		passwordEnv := target.EnvMySQLPassword
		if k != config.Default {
			passwordEnv = passwordEnv + config.Default + k
		}
		databaseEnv := target.EnvMySQLDatabase
		if k != config.Default {
			databaseEnv = databaseEnv + config.Default + k
		}
		queueDirEnv := target.EnvMySQLQueueDir
		if k != config.Default {
			queueDirEnv = queueDirEnv + config.Default + k
		}
		mysqlArgs := target.MySQLArgs{
			Enable:     enabled,
			Format:     env.Get(formatEnv, kv.Get(target.MySQLFormat)),
			DSN:        env.Get(dsnStringEnv, kv.Get(target.MySQLDSNString)),
			Table:      env.Get(tableEnv, kv.Get(target.MySQLTable)),
			Host:       *host,
			Port:       env.Get(portEnv, kv.Get(target.MySQLPort)),
			User:       env.Get(usernameEnv, kv.Get(target.MySQLUsername)),
			Password:   env.Get(passwordEnv, kv.Get(target.MySQLPassword)),
			Database:   env.Get(databaseEnv, kv.Get(target.MySQLDatabase)),
			QueueDir:   env.Get(queueDirEnv, kv.Get(target.MySQLQueueDir)),
			QueueLimit: queueLimit,
		}
		if err = mysqlArgs.Validate(); err != nil {
			return nil, err
		}
		mysqlTargets[k] = mysqlArgs
	}
	return mysqlTargets, nil
}

// DefaultNATSKVS - NATS KV for nats config.
var (
	DefaultNATSKVS = config.KVS{
		config.State:                           config.StateOff,
		config.Comment:                         "Default settings for NATS notification",
		target.NATSAddress:                     "",
		target.NATSSubject:                     "",
		target.NATSUsername:                    "",
		target.NATSPassword:                    "",
		target.NATSToken:                       "",
		target.NATSSecure:                      config.StateOff,
		target.NATSPingInterval:                "0",
		target.NATSQueueLimit:                  "0",
		target.NATSQueueDir:                    "",
		target.NATSStreamingEnable:             config.StateOff,
		target.NATSStreamingAsync:              config.StateOff,
		target.NATSStreamingMaxPubAcksInFlight: "0",
		target.NATSStreamingClusterID:          "",
	}
)

// GetNotifyNATS - returns a map of registered notification 'nats' targets
func GetNotifyNATS(natsKVS map[string]config.KVS) (map[string]target.NATSArgs, error) {
	natsTargets := make(map[string]target.NATSArgs)
	for k, kv := range mergeTargets(natsKVS, target.EnvNATSState, DefaultNATSKVS) {
		stateEnv := target.EnvNATSState
		if k != config.Default {
			stateEnv = stateEnv + config.Default + k
		}

		enabled, err := config.ParseBool(env.Get(stateEnv, kv.Get(config.State)))
		if err != nil {
			return nil, err
		}
		if !enabled {
			continue
		}

		addressEnv := target.EnvNATSAddress
		if k != config.Default {
			addressEnv = addressEnv + config.Default + k
		}

		address, err := xnet.ParseHost(env.Get(addressEnv, kv.Get(target.NATSAddress)))
		if err != nil {
			return nil, err
		}

		pingIntervalEnv := target.EnvNATSPingInterval
		if k != config.Default {
			pingIntervalEnv = pingIntervalEnv + config.Default + k
		}

		pingInterval, err := strconv.ParseInt(env.Get(pingIntervalEnv, kv.Get(target.NATSPingInterval)), 10, 64)
		if err != nil {
			return nil, err
		}

		queueLimitEnv := target.EnvNATSQueueLimit
		if k != config.Default {
			queueLimitEnv = queueLimitEnv + config.Default + k
		}

		queueLimit, err := strconv.ParseUint(env.Get(queueLimitEnv, kv.Get(target.NATSQueueLimit)), 10, 64)
		if err != nil {
			return nil, err
		}

		secureEnv := target.EnvNATSSecure
		if k != config.Default {
			secureEnv = secureEnv + config.Default + k
		}

		subjectEnv := target.EnvNATSSubject
		if k != config.Default {
			subjectEnv = subjectEnv + config.Default + k
		}

		usernameEnv := target.EnvNATSUsername
		if k != config.Default {
			usernameEnv = usernameEnv + config.Default + k
		}

		passwordEnv := target.EnvNATSPassword
		if k != config.Default {
			passwordEnv = passwordEnv + config.Default + k
		}

		tokenEnv := target.EnvNATSToken
		if k != config.Default {
			tokenEnv = tokenEnv + config.Default + k
		}

		queueDirEnv := target.EnvNATSQueueDir
		if k != config.Default {
			queueDirEnv = queueDirEnv + config.Default + k
		}

		natsArgs := target.NATSArgs{
			Enable:       true,
			Address:      *address,
			Subject:      env.Get(subjectEnv, kv.Get(target.NATSSubject)),
			Username:     env.Get(usernameEnv, kv.Get(target.NATSUsername)),
			Password:     env.Get(passwordEnv, kv.Get(target.NATSPassword)),
			Token:        env.Get(tokenEnv, kv.Get(target.NATSToken)),
			Secure:       env.Get(secureEnv, kv.Get(target.NATSSecure)) == config.StateOn,
			PingInterval: pingInterval,
			QueueDir:     env.Get(queueDirEnv, kv.Get(target.NATSQueueDir)),
			QueueLimit:   queueLimit,
		}

		streamingEnableEnv := target.EnvNATSStreamingEnable
		if k != config.Default {
			streamingEnableEnv = streamingEnableEnv + config.Default + k
		}

		streamingEnabled := env.Get(streamingEnableEnv, kv.Get(target.NATSStreamingEnable)) == config.StateOn
		if streamingEnabled {
			asyncEnv := target.EnvNATSStreamingAsync
			if k != config.Default {
				asyncEnv = asyncEnv + config.Default + k
			}
			maxPubAcksInflightEnv := target.EnvNATSStreamingMaxPubAcksInFlight
			if k != config.Default {
				maxPubAcksInflightEnv = maxPubAcksInflightEnv + config.Default + k
			}
			maxPubAcksInflight, err := strconv.Atoi(env.Get(maxPubAcksInflightEnv,
				kv.Get(target.NATSStreamingMaxPubAcksInFlight)))
			if err != nil {
				return nil, err
			}
			clusterIDEnv := target.EnvNATSStreamingClusterID
			if k != config.Default {
				clusterIDEnv = clusterIDEnv + config.Default + k
			}
			natsArgs.Streaming.Enable = streamingEnabled
			natsArgs.Streaming.ClusterID = env.Get(clusterIDEnv, kv.Get(target.NATSStreamingClusterID))
			natsArgs.Streaming.Async = env.Get(asyncEnv, kv.Get(target.NATSStreamingAsync)) == config.StateOn
			natsArgs.Streaming.MaxPubAcksInflight = maxPubAcksInflight
		}

		if err = natsArgs.Validate(); err != nil {
			return nil, err
		}

		natsTargets[k] = natsArgs
	}
	return natsTargets, nil
}

// DefaultNSQKVS - NSQ KV for config
var (
	DefaultNSQKVS = config.KVS{
		config.State:            config.StateOff,
		config.Comment:          "Default settings for NSQ notification",
		target.NSQAddress:       "",
		target.NSQTopic:         "",
		target.NSQTLSEnable:     config.StateOff,
		target.NSQTLSSkipVerify: config.StateOff,
		target.NSQQueueLimit:    "0",
		target.NSQQueueDir:      "",
	}
)

// GetNotifyNSQ - returns a map of registered notification 'nsq' targets
func GetNotifyNSQ(nsqKVS map[string]config.KVS) (map[string]target.NSQArgs, error) {
	nsqTargets := make(map[string]target.NSQArgs)
	for k, kv := range mergeTargets(nsqKVS, target.EnvNSQState, DefaultNSQKVS) {
		stateEnv := target.EnvNSQState
		if k != config.Default {
			stateEnv = stateEnv + config.Default + k
		}

		enabled, err := config.ParseBool(env.Get(stateEnv, kv.Get(config.State)))
		if err != nil {
			return nil, err
		}
		if !enabled {
			continue
		}

		addressEnv := target.EnvNSQAddress
		if k != config.Default {
			addressEnv = addressEnv + config.Default + k
		}
		nsqdAddress, err := xnet.ParseHost(env.Get(addressEnv, kv.Get(target.NSQAddress)))
		if err != nil {
			return nil, err
		}
		tlsEnableEnv := target.EnvNSQTLSEnable
		if k != config.Default {
			tlsEnableEnv = tlsEnableEnv + config.Default + k
		}
		tlsSkipVerifyEnv := target.EnvNSQTLSSkipVerify
		if k != config.Default {
			tlsSkipVerifyEnv = tlsSkipVerifyEnv + config.Default + k
		}

		queueLimitEnv := target.EnvNSQQueueLimit
		if k != config.Default {
			queueLimitEnv = queueLimitEnv + config.Default + k
		}
		queueLimit, err := strconv.ParseUint(env.Get(queueLimitEnv, kv.Get(target.NSQQueueLimit)), 10, 64)
		if err != nil {
			return nil, err
		}

		topicEnv := target.EnvNSQTopic
		if k != config.Default {
			topicEnv = topicEnv + config.Default + k
		}
		queueDirEnv := target.EnvNSQQueueDir
		if k != config.Default {
			queueDirEnv = queueDirEnv + config.Default + k
		}

		nsqArgs := target.NSQArgs{
			Enable:      enabled,
			NSQDAddress: *nsqdAddress,
			Topic:       env.Get(topicEnv, kv.Get(target.NSQTopic)),
			QueueDir:    env.Get(queueDirEnv, kv.Get(target.NSQQueueDir)),
			QueueLimit:  queueLimit,
		}
		nsqArgs.TLS.Enable = env.Get(tlsEnableEnv, kv.Get(target.NSQTLSEnable)) == config.StateOn
		nsqArgs.TLS.SkipVerify = env.Get(tlsSkipVerifyEnv, kv.Get(target.NSQTLSSkipVerify)) == config.StateOn

		if err = nsqArgs.Validate(); err != nil {
			return nil, err
		}

		nsqTargets[k] = nsqArgs
	}
	return nsqTargets, nil
}

// DefaultPostgresKVS - default Postgres KV for server config.
var (
	DefaultPostgresKVS = config.KVS{
		config.State:                    config.StateOff,
		config.Comment:                  "Default settings for Postgres notification",
		target.PostgresFormat:           formatNamespace,
		target.PostgresConnectionString: "",
		target.PostgresTable:            "",
		target.PostgresHost:             "",
		target.PostgresPort:             "",
		target.PostgresUsername:         "",
		target.PostgresPassword:         "",
		target.PostgresDatabase:         "",
		target.PostgresQueueDir:         "",
		target.PostgresQueueLimit:       "0",
	}
)

// GetNotifyPostgres - returns a map of registered notification 'postgres' targets
func GetNotifyPostgres(postgresKVS map[string]config.KVS) (map[string]target.PostgreSQLArgs, error) {
	psqlTargets := make(map[string]target.PostgreSQLArgs)
	for k, kv := range mergeTargets(postgresKVS, target.EnvPostgresState, DefaultPostgresKVS) {
		stateEnv := target.EnvPostgresState
		if k != config.Default {
			stateEnv = stateEnv + config.Default + k
		}

		enabled, err := config.ParseBool(env.Get(stateEnv, kv.Get(config.State)))
		if err != nil {
			return nil, err
		}
		if !enabled {
			continue
		}

		hostEnv := target.EnvPostgresHost
		if k != config.Default {
			hostEnv = hostEnv + config.Default + k
		}

		host, err := xnet.ParseHost(env.Get(hostEnv, kv.Get(target.PostgresHost)))
		if err != nil {
			return nil, err
		}

		queueLimitEnv := target.EnvPostgresQueueLimit
		if k != config.Default {
			queueLimitEnv = queueLimitEnv + config.Default + k
		}

		queueLimit, err := strconv.Atoi(env.Get(queueLimitEnv, kv.Get(target.PostgresQueueLimit)))
		if err != nil {
			return nil, err
		}

		formatEnv := target.EnvPostgresFormat
		if k != config.Default {
			formatEnv = formatEnv + config.Default + k
		}

		connectionStringEnv := target.EnvPostgresConnectionString
		if k != config.Default {
			connectionStringEnv = connectionStringEnv + config.Default + k
		}

		tableEnv := target.EnvPostgresTable
		if k != config.Default {
			tableEnv = tableEnv + config.Default + k
		}

		portEnv := target.EnvPostgresPort
		if k != config.Default {
			portEnv = portEnv + config.Default + k
		}

		usernameEnv := target.EnvPostgresUsername
		if k != config.Default {
			usernameEnv = usernameEnv + config.Default + k
		}

		passwordEnv := target.EnvPostgresPassword
		if k != config.Default {
			passwordEnv = passwordEnv + config.Default + k
		}

		databaseEnv := target.EnvPostgresDatabase
		if k != config.Default {
			databaseEnv = databaseEnv + config.Default + k
		}

		queueDirEnv := target.EnvPostgresQueueDir
		if k != config.Default {
			queueDirEnv = queueDirEnv + config.Default + k
		}

		psqlArgs := target.PostgreSQLArgs{
			Enable:           enabled,
			Format:           env.Get(formatEnv, kv.Get(target.PostgresFormat)),
			ConnectionString: env.Get(connectionStringEnv, kv.Get(target.PostgresConnectionString)),
			Table:            env.Get(tableEnv, kv.Get(target.PostgresTable)),
			Host:             *host,
			Port:             env.Get(portEnv, kv.Get(target.PostgresPort)),
			User:             env.Get(usernameEnv, kv.Get(target.PostgresUsername)),
			Password:         env.Get(passwordEnv, kv.Get(target.PostgresPassword)),
			Database:         env.Get(databaseEnv, kv.Get(target.PostgresDatabase)),
			QueueDir:         env.Get(queueDirEnv, kv.Get(target.PostgresQueueDir)),
			QueueLimit:       uint64(queueLimit),
		}
		if err = psqlArgs.Validate(); err != nil {
			return nil, err
		}
		psqlTargets[k] = psqlArgs
	}

	return psqlTargets, nil
}

// DefaultRedisKVS - default KV for redis config
var (
	DefaultRedisKVS = config.KVS{
		config.State:           config.StateOff,
		config.Comment:         "Default settings for Redis notification",
		target.RedisFormat:     formatNamespace,
		target.RedisAddress:    "",
		target.RedisKey:        "",
		target.RedisPassword:   "",
		target.RedisQueueDir:   "",
		target.RedisQueueLimit: "0",
	}
)

// GetNotifyRedis - returns a map of registered notification 'redis' targets
func GetNotifyRedis(redisKVS map[string]config.KVS) (map[string]target.RedisArgs, error) {
	redisTargets := make(map[string]target.RedisArgs)
	for k, kv := range mergeTargets(redisKVS, target.EnvRedisState, DefaultRedisKVS) {
		stateEnv := target.EnvRedisState
		if k != config.Default {
			stateEnv = stateEnv + config.Default + k
		}

		enabled, err := config.ParseBool(env.Get(stateEnv, kv.Get(config.State)))
		if err != nil {
			return nil, err
		}
		if !enabled {
			continue
		}

		addressEnv := target.EnvRedisAddress
		if k != config.Default {
			addressEnv = addressEnv + config.Default + k
		}
		addr, err := xnet.ParseHost(env.Get(addressEnv, kv.Get(target.RedisAddress)))
		if err != nil {
			return nil, err
		}
		queueLimitEnv := target.EnvRedisQueueLimit
		if k != config.Default {
			queueLimitEnv = queueLimitEnv + config.Default + k
		}
		queueLimit, err := strconv.Atoi(env.Get(queueLimitEnv, kv.Get(target.RedisQueueLimit)))
		if err != nil {
			return nil, err
		}
		formatEnv := target.EnvRedisFormat
		if k != config.Default {
			formatEnv = formatEnv + config.Default + k
		}
		passwordEnv := target.EnvRedisPassword
		if k != config.Default {
			passwordEnv = passwordEnv + config.Default + k
		}
		keyEnv := target.EnvRedisKey
		if k != config.Default {
			keyEnv = keyEnv + config.Default + k
		}
		queueDirEnv := target.EnvRedisQueueDir
		if k != config.Default {
			queueDirEnv = queueDirEnv + config.Default + k
		}
		redisArgs := target.RedisArgs{
			Enable:     enabled,
			Format:     env.Get(formatEnv, kv.Get(target.RedisFormat)),
			Addr:       *addr,
			Password:   env.Get(passwordEnv, kv.Get(target.RedisPassword)),
			Key:        env.Get(keyEnv, kv.Get(target.RedisKey)),
			QueueDir:   env.Get(queueDirEnv, kv.Get(target.RedisQueueDir)),
			QueueLimit: uint64(queueLimit),
		}
		if err = redisArgs.Validate(); err != nil {
			return nil, err
		}
		redisTargets[k] = redisArgs
	}
	return redisTargets, nil
}

// DefaultWebhookKVS - default KV for webhook config
var (
	DefaultWebhookKVS = config.KVS{
		config.State:             config.StateOff,
		config.Comment:           "Default settings for Webhook notification",
		target.WebhookEndpoint:   "",
		target.WebhookAuthToken:  "",
		target.WebhookQueueLimit: "0",
		target.WebhookQueueDir:   "",
	}
)

// GetNotifyWebhook - returns a map of registered notification 'webhook' targets
func GetNotifyWebhook(webhookKVS map[string]config.KVS, rootCAs *x509.CertPool) (map[string]target.WebhookArgs, error) {
	webhookTargets := make(map[string]target.WebhookArgs)
	for k, kv := range mergeTargets(webhookKVS, target.EnvWebhookState, DefaultWebhookKVS) {
		stateEnv := target.EnvWebhookState
		if k != config.Default {
			stateEnv = stateEnv + config.Default + k
		}
		enabled, err := config.ParseBool(env.Get(stateEnv, kv.Get(config.State)))
		if err != nil {
			return nil, err
		}
		if !enabled {
			continue
		}
		urlEnv := target.EnvWebhookEndpoint
		if k != config.Default {
			urlEnv = urlEnv + config.Default + k
		}
		url, err := xnet.ParseHTTPURL(env.Get(urlEnv, kv.Get(target.WebhookEndpoint)))
		if err != nil {
			return nil, err
		}
		queueLimitEnv := target.EnvWebhookQueueLimit
		if k != config.Default {
			queueLimitEnv = queueLimitEnv + config.Default + k
		}
		queueLimit, err := strconv.Atoi(env.Get(queueLimitEnv, kv.Get(target.WebhookQueueLimit)))
		if err != nil {
			return nil, err
		}
		queueDirEnv := target.EnvWebhookQueueDir
		if k != config.Default {
			queueDirEnv = queueDirEnv + config.Default + k
		}
		authEnv := target.EnvWebhookAuthToken
		if k != config.Default {
			authEnv = authEnv + config.Default + k
		}

		webhookArgs := target.WebhookArgs{
			Enable:     enabled,
			Endpoint:   *url,
			RootCAs:    rootCAs,
			AuthToken:  env.Get(authEnv, kv.Get(target.WebhookAuthToken)),
			QueueDir:   env.Get(queueDirEnv, kv.Get(target.WebhookQueueDir)),
			QueueLimit: uint64(queueLimit),
		}
		if err = webhookArgs.Validate(); err != nil {
			return nil, err
		}
		webhookTargets[k] = webhookArgs
	}
	return webhookTargets, nil
}

// DefaultESKVS - default KV config for Elasticsearch target
var (
	DefaultESKVS = config.KVS{
		config.State:             config.StateOff,
		config.Comment:           "Default settings for Elasticsearch notification",
		target.ElasticURL:        "",
		target.ElasticFormat:     formatNamespace,
		target.ElasticIndex:      "",
		target.ElasticQueueDir:   "",
		target.ElasticQueueLimit: "0",
	}
)

// GetNotifyES - returns a map of registered notification 'elasticsearch' targets
func GetNotifyES(esKVS map[string]config.KVS) (map[string]target.ElasticsearchArgs, error) {
	esTargets := make(map[string]target.ElasticsearchArgs)
	for k, kv := range mergeTargets(esKVS, target.EnvElasticState, DefaultESKVS) {
		stateEnv := target.EnvElasticState
		if k != config.Default {
			stateEnv = stateEnv + config.Default + k
		}
		enabled, err := config.ParseBool(env.Get(stateEnv, kv.Get(config.State)))
		if err != nil {
			return nil, err
		}
		if !enabled {
			continue
		}

		urlEnv := target.EnvElasticURL
		if k != config.Default {
			urlEnv = urlEnv + config.Default + k
		}

		url, err := xnet.ParseHTTPURL(env.Get(urlEnv, kv.Get(target.ElasticURL)))
		if err != nil {
			return nil, err
		}

		queueLimitEnv := target.EnvElasticQueueLimit
		if k != config.Default {
			queueLimitEnv = queueLimitEnv + config.Default + k
		}

		queueLimit, err := strconv.Atoi(env.Get(queueLimitEnv, kv.Get(target.ElasticQueueLimit)))
		if err != nil {
			return nil, err
		}

		formatEnv := target.EnvElasticFormat
		if k != config.Default {
			formatEnv = formatEnv + config.Default + k
		}

		indexEnv := target.EnvElasticIndex
		if k != config.Default {
			indexEnv = indexEnv + config.Default + k
		}

		queueDirEnv := target.EnvElasticQueueDir
		if k != config.Default {
			queueDirEnv = queueDirEnv + config.Default + k
		}

		esArgs := target.ElasticsearchArgs{
			Enable:     enabled,
			Format:     env.Get(formatEnv, kv.Get(target.ElasticFormat)),
			URL:        *url,
			Index:      env.Get(indexEnv, kv.Get(target.ElasticIndex)),
			QueueDir:   env.Get(queueDirEnv, kv.Get(target.ElasticQueueDir)),
			QueueLimit: uint64(queueLimit),
		}
		if err = esArgs.Validate(); err != nil {
			return nil, err
		}
		esTargets[k] = esArgs
	}
	return esTargets, nil
}

// DefaultAMQPKVS - default KV for AMQP config
var (
	DefaultAMQPKVS = config.KVS{
		config.State:            config.StateOff,
		config.Comment:          "Default settings for AMQP notification",
		target.AmqpURL:          "",
		target.AmqpExchange:     "",
		target.AmqpExchangeType: "",
		target.AmqpRoutingKey:   "",
		target.AmqpMandatory:    config.StateOff,
		target.AmqpDurable:      config.StateOff,
		target.AmqpNoWait:       config.StateOff,
		target.AmqpInternal:     config.StateOff,
		target.AmqpAutoDeleted:  config.StateOff,
		target.AmqpDeliveryMode: "0",
		target.AmqpQueueLimit:   "0",
		target.AmqpQueueDir:     "",
	}
)

// GetNotifyAMQP - returns a map of registered notification 'amqp' targets
func GetNotifyAMQP(amqpKVS map[string]config.KVS) (map[string]target.AMQPArgs, error) {
	amqpTargets := make(map[string]target.AMQPArgs)
	for k, kv := range mergeTargets(amqpKVS, target.EnvAMQPState, DefaultAMQPKVS) {
		stateEnv := target.EnvAMQPState
		if k != config.Default {
			stateEnv = stateEnv + config.Default + k
		}
		enabled, err := config.ParseBool(env.Get(stateEnv, kv.Get(config.State)))
		if err != nil {
			return nil, err
		}
		if !enabled {
			continue
		}
		urlEnv := target.EnvAMQPURL
		if k != config.Default {
			urlEnv = urlEnv + config.Default + k
		}
		url, err := xnet.ParseURL(env.Get(urlEnv, kv.Get(target.AmqpURL)))
		if err != nil {
			return nil, err
		}
		deliveryModeEnv := target.EnvAMQPDeliveryMode
		if k != config.Default {
			deliveryModeEnv = deliveryModeEnv + config.Default + k
		}
		deliveryMode, err := strconv.Atoi(env.Get(deliveryModeEnv, kv.Get(target.AmqpDeliveryMode)))
		if err != nil {
			return nil, err
		}
		exchangeEnv := target.EnvAMQPExchange
		if k != config.Default {
			exchangeEnv = exchangeEnv + config.Default + k
		}
		routingKeyEnv := target.EnvAMQPRoutingKey
		if k != config.Default {
			routingKeyEnv = routingKeyEnv + config.Default + k
		}
		exchangeTypeEnv := target.EnvAMQPExchangeType
		if k != config.Default {
			exchangeTypeEnv = exchangeTypeEnv + config.Default + k
		}
		mandatoryEnv := target.EnvAMQPMandatory
		if k != config.Default {
			mandatoryEnv = mandatoryEnv + config.Default + k
		}
		immediateEnv := target.EnvAMQPImmediate
		if k != config.Default {
			immediateEnv = immediateEnv + config.Default + k
		}
		durableEnv := target.EnvAMQPDurable
		if k != config.Default {
			durableEnv = durableEnv + config.Default + k
		}
		internalEnv := target.EnvAMQPInternal
		if k != config.Default {
			internalEnv = internalEnv + config.Default + k
		}
		noWaitEnv := target.EnvAMQPNoWait
		if k != config.Default {
			noWaitEnv = noWaitEnv + config.Default + k
		}
		autoDeletedEnv := target.EnvAMQPAutoDeleted
		if k != config.Default {
			autoDeletedEnv = autoDeletedEnv + config.Default + k
		}
		queueDirEnv := target.EnvAMQPQueueDir
		if k != config.Default {
			queueDirEnv = queueDirEnv + config.Default + k
		}
		queueLimitEnv := target.EnvAMQPQueueLimit
		if k != config.Default {
			queueLimitEnv = queueLimitEnv + config.Default + k
		}
		queueLimit, err := strconv.ParseUint(env.Get(queueLimitEnv, kv.Get(target.AmqpQueueLimit)), 10, 64)
		if err != nil {
			return nil, err
		}
		amqpArgs := target.AMQPArgs{
			Enable:       enabled,
			URL:          *url,
			Exchange:     env.Get(exchangeEnv, kv.Get(target.AmqpExchange)),
			RoutingKey:   env.Get(routingKeyEnv, kv.Get(target.AmqpRoutingKey)),
			ExchangeType: env.Get(exchangeTypeEnv, kv.Get(target.AmqpExchangeType)),
			DeliveryMode: uint8(deliveryMode),
			Mandatory:    env.Get(mandatoryEnv, kv.Get(target.AmqpMandatory)) == config.StateOn,
			Immediate:    env.Get(immediateEnv, kv.Get(target.AmqpImmediate)) == config.StateOn,
			Durable:      env.Get(durableEnv, kv.Get(target.AmqpDurable)) == config.StateOn,
			Internal:     env.Get(internalEnv, kv.Get(target.AmqpInternal)) == config.StateOn,
			NoWait:       env.Get(noWaitEnv, kv.Get(target.AmqpNoWait)) == config.StateOn,
			AutoDeleted:  env.Get(autoDeletedEnv, kv.Get(target.AmqpAutoDeleted)) == config.StateOn,
			QueueDir:     env.Get(queueDirEnv, kv.Get(target.AmqpQueueDir)),
			QueueLimit:   queueLimit,
		}
		if err = amqpArgs.Validate(); err != nil {
			return nil, err
		}
		amqpTargets[k] = amqpArgs
	}
	return amqpTargets, nil
}
