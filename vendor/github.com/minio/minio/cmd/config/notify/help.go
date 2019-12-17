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
	"github.com/minio/minio/cmd/config"
	"github.com/minio/minio/pkg/event/target"
)

const (
	formatComment     = `'namespace' reflects current bucket/object list and 'access' reflects a journal of object operations, defaults to 'namespace'`
	queueDirComment   = `staging dir for undelivered messages e.g. '/home/events'`
	queueLimitComment = `maximum limit for undelivered messages, defaults to '10000'`
)

// Help template inputs for all notification targets
var (
	HelpWebhook = config.HelpKVS{
		config.HelpKV{
			Key:         target.WebhookEndpoint,
			Description: "webhook server endpoint e.g. http://localhost:8080/minio/events",
			Type:        "url",
		},
		config.HelpKV{
			Key:         target.WebhookAuthToken,
			Description: "opaque string or JWT authorization token",
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.WebhookQueueDir,
			Description: queueDirComment,
			Optional:    true,
			Type:        "path",
		},
		config.HelpKV{
			Key:         target.WebhookQueueLimit,
			Description: queueLimitComment,
			Optional:    true,
			Type:        "number",
		},
		config.HelpKV{
			Key:         config.Comment,
			Description: config.DefaultComment,
			Optional:    true,
			Type:        "sentence",
		},
	}

	HelpAMQP = config.HelpKVS{
		config.HelpKV{
			Key:         target.AmqpURL,
			Description: "AMQP server endpoint e.g. `amqp://myuser:mypassword@localhost:5672`",
			Type:        "url",
		},
		config.HelpKV{
			Key:         target.AmqpExchange,
			Description: "name of the AMQP exchange",
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.AmqpExchangeType,
			Description: "AMQP exchange type",
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.AmqpRoutingKey,
			Description: "routing key for publishing",
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.AmqpMandatory,
			Description: "quietly ignore undelivered messages when set to 'off', default is 'on'",
			Optional:    true,
			Type:        "on|off",
		},
		config.HelpKV{
			Key:         target.AmqpDurable,
			Description: "persist queue across broker restarts when set to 'on', default is 'off'",
			Optional:    true,
			Type:        "on|off",
		},
		config.HelpKV{
			Key:         target.AmqpNoWait,
			Description: "non-blocking message delivery when set to 'on', default is 'off'",
			Optional:    true,
			Type:        "on|off",
		},
		config.HelpKV{
			Key:         target.AmqpInternal,
			Description: "set to 'on' for exchange to be not used directly by publishers, but only when bound to other exchanges",
			Optional:    true,
			Type:        "on|off",
		},
		config.HelpKV{
			Key:         target.AmqpAutoDeleted,
			Description: "auto delete queue when set to 'on', when there are no consumers",
			Optional:    true,
			Type:        "on|off",
		},
		config.HelpKV{
			Key:         target.AmqpDeliveryMode,
			Description: "set to '1' for non-persistent or '2' for persistent queue",
			Optional:    true,
			Type:        "number",
		},
		config.HelpKV{
			Key:         target.AmqpQueueDir,
			Description: queueDirComment,
			Optional:    true,
			Type:        "path",
		},
		config.HelpKV{
			Key:         target.AmqpQueueLimit,
			Description: queueLimitComment,
			Optional:    true,
			Type:        "number",
		},
		config.HelpKV{
			Key:         config.Comment,
			Description: config.DefaultComment,
			Optional:    true,
			Type:        "sentence",
		},
	}

	HelpKafka = config.HelpKVS{
		config.HelpKV{
			Key:         target.KafkaBrokers,
			Description: "comma separated list of Kafka broker addresses",
			Type:        "csv",
		},
		config.HelpKV{
			Key:         target.KafkaTopic,
			Description: "Kafka topic used for bucket notifications",
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.KafkaSASLUsername,
			Description: "username for SASL/PLAIN or SASL/SCRAM authentication",
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.KafkaSASLPassword,
			Description: "password for SASL/PLAIN or SASL/SCRAM authentication",
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.KafkaTLSClientAuth,
			Description: "clientAuth determines the Kafka server's policy for TLS client auth",
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.KafkaSASL,
			Description: "set to 'on' to enable SASL authentication",
			Optional:    true,
			Type:        "on|off",
		},
		config.HelpKV{
			Key:         target.KafkaTLS,
			Description: "set to 'on' to enable TLS",
			Optional:    true,
			Type:        "on|off",
		},
		config.HelpKV{
			Key:         target.KafkaTLSSkipVerify,
			Description: `trust server TLS without verification, defaults to "on" (verify)`,
			Optional:    true,
			Type:        "on|off",
		},
		config.HelpKV{
			Key:         target.KafkaClientTLSCert,
			Description: "path to client certificate for mTLS auth",
			Optional:    true,
			Type:        "path",
		},
		config.HelpKV{
			Key:         target.KafkaClientTLSKey,
			Description: "path to client key for mTLS auth",
			Optional:    true,
			Type:        "path",
		},
		config.HelpKV{
			Key:         target.KafkaQueueDir,
			Description: queueDirComment,
			Optional:    true,
			Type:        "path",
		},
		config.HelpKV{
			Key:         target.KafkaQueueLimit,
			Description: queueLimitComment,
			Optional:    true,
			Type:        "number",
		},
		config.HelpKV{
			Key:         config.Comment,
			Description: config.DefaultComment,
			Optional:    true,
			Type:        "sentence",
		},
	}

	HelpMQTT = config.HelpKVS{
		config.HelpKV{
			Key:         target.MqttBroker,
			Description: "MQTT server endpoint e.g. `tcp://localhost:1883`",
			Type:        "uri",
		},
		config.HelpKV{
			Key:         target.MqttTopic,
			Description: "name of the MQTT topic to publish",
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.MqttUsername,
			Description: "MQTT username",
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.MqttPassword,
			Description: "MQTT password",
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.MqttQoS,
			Description: "set the quality of service priority, defaults to '0'",
			Optional:    true,
			Type:        "number",
		},
		config.HelpKV{
			Key:         target.MqttKeepAliveInterval,
			Description: "keep-alive interval for MQTT connections in s,m,h,d",
			Optional:    true,
			Type:        "duration",
		},
		config.HelpKV{
			Key:         target.MqttReconnectInterval,
			Description: "reconnect interval for MQTT connections in s,m,h,d",
			Optional:    true,
			Type:        "duration",
		},
		config.HelpKV{
			Key:         target.MqttQueueDir,
			Description: queueDirComment,
			Optional:    true,
			Type:        "path",
		},
		config.HelpKV{
			Key:         target.MqttQueueLimit,
			Description: queueLimitComment,
			Optional:    true,
			Type:        "number",
		},
		config.HelpKV{
			Key:         config.Comment,
			Description: config.DefaultComment,
			Optional:    true,
			Type:        "sentence",
		},
	}

	HelpPostgres = config.HelpKVS{
		config.HelpKV{
			Key:         target.PostgresConnectionString,
			Description: "Postgres server connection-string",
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.PostgresTable,
			Description: "DB table name to store/update events, table is auto-created",
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.PostgresFormat,
			Description: formatComment,
			Type:        "namespace*|access",
		},
		config.HelpKV{
			Key:         target.PostgresHost,
			Description: "Postgres server hostname (used only if `connection_string` is empty)",
			Optional:    true,
			Type:        "hostname",
		},
		config.HelpKV{
			Key:         target.PostgresPort,
			Description: "Postgres server port, defaults to `5432` (used only if `connection_string` is empty)",
			Optional:    true,
			Type:        "port",
		},
		config.HelpKV{
			Key:         target.PostgresUsername,
			Description: "database username (used only if `connection_string` is empty)",
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.PostgresPassword,
			Description: "database password (used only if `connection_string` is empty)",
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.PostgresDatabase,
			Description: "database name (used only if `connection_string` is empty)",
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.PostgresQueueDir,
			Description: queueDirComment,
			Optional:    true,
			Type:        "path",
		},
		config.HelpKV{
			Key:         target.PostgresQueueLimit,
			Description: queueLimitComment,
			Optional:    true,
			Type:        "number",
		},
		config.HelpKV{
			Key:         config.Comment,
			Description: config.DefaultComment,
			Optional:    true,
			Type:        "sentence",
		},
	}

	HelpMySQL = config.HelpKVS{
		config.HelpKV{
			Key:         target.MySQLDSNString,
			Description: "MySQL data-source-name connection string",
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.MySQLTable,
			Description: "DB table name to store/update events, table is auto-created",
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.MySQLFormat,
			Description: formatComment,
			Type:        "namespace*|access",
		},
		config.HelpKV{
			Key:         target.MySQLHost,
			Description: "MySQL server hostname (used only if `dsn_string` is empty)",
			Optional:    true,
			Type:        "hostname",
		},
		config.HelpKV{
			Key:         target.MySQLPort,
			Description: "MySQL server port (used only if `dsn_string` is empty)",
			Optional:    true,
			Type:        "port",
		},
		config.HelpKV{
			Key:         target.MySQLUsername,
			Description: "database username (used only if `dsn_string` is empty)",
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.MySQLPassword,
			Description: "database password (used only if `dsn_string` is empty)",
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.MySQLDatabase,
			Description: "database name (used only if `dsn_string` is empty)",
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.MySQLQueueDir,
			Description: queueDirComment,
			Optional:    true,
			Type:        "path",
		},
		config.HelpKV{
			Key:         target.MySQLQueueLimit,
			Description: queueLimitComment,
			Optional:    true,
			Type:        "number",
		},
		config.HelpKV{
			Key:         config.Comment,
			Description: config.DefaultComment,
			Optional:    true,
			Type:        "sentence",
		},
	}

	HelpNATS = config.HelpKVS{
		config.HelpKV{
			Key:         target.NATSAddress,
			Description: "NATS server address e.g. '0.0.0.0:4222'",
			Type:        "address",
		},
		config.HelpKV{
			Key:         target.NATSSubject,
			Description: "NATS subscription subject",
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.NATSUsername,
			Description: "NATS username",
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.NATSPassword,
			Description: "NATS password",
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.NATSToken,
			Description: "NATS token",
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.NATSTLS,
			Description: "set to 'on' to enable TLS",
			Optional:    true,
			Type:        "on|off",
		},
		config.HelpKV{
			Key:         target.NATSTLSSkipVerify,
			Description: `trust server TLS without verification, defaults to "on" (verify)`,
			Optional:    true,
			Type:        "on|off",
		},
		config.HelpKV{
			Key:         target.NATSPingInterval,
			Description: "client ping commands interval in s,m,h,d. Disabled by default",
			Optional:    true,
			Type:        "duration",
		},
		config.HelpKV{
			Key:         target.NATSStreaming,
			Description: "set to 'on', to use streaming NATS server",
			Optional:    true,
			Type:        "on|off",
		},
		config.HelpKV{
			Key:         target.NATSStreamingAsync,
			Description: "set to 'on', to enable asynchronous publish",
			Optional:    true,
			Type:        "on|off",
		},
		config.HelpKV{
			Key:         target.NATSStreamingMaxPubAcksInFlight,
			Description: "number of messages to publish without waiting for ACKs",
			Optional:    true,
			Type:        "number",
		},
		config.HelpKV{
			Key:         target.NATSStreamingClusterID,
			Description: "unique ID for NATS streaming cluster",
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.NATSCertAuthority,
			Description: "path to certificate chain of the target NATS server",
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.NATSClientCert,
			Description: "client cert for NATS mTLS auth",
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.NATSClientKey,
			Description: "client cert key for NATS mTLS auth",
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.NATSQueueDir,
			Description: queueDirComment,
			Optional:    true,
			Type:        "path",
		},
		config.HelpKV{
			Key:         target.NATSQueueLimit,
			Description: queueLimitComment,
			Optional:    true,
			Type:        "number",
		},
		config.HelpKV{
			Key:         config.Comment,
			Description: config.DefaultComment,
			Optional:    true,
			Type:        "sentence",
		},
	}

	HelpNSQ = config.HelpKVS{
		config.HelpKV{
			Key:         target.NSQAddress,
			Description: "NSQ server address e.g. '127.0.0.1:4150'",
			Type:        "address",
		},
		config.HelpKV{
			Key:         target.NSQTopic,
			Description: "NSQ topic",
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.NSQTLS,
			Description: "set to 'on' to enable TLS",
			Optional:    true,
			Type:        "on|off",
		},
		config.HelpKV{
			Key:         target.NSQTLSSkipVerify,
			Description: `trust server TLS without verification, defaults to "on" (verify)`,
			Optional:    true,
			Type:        "on|off",
		},
		config.HelpKV{
			Key:         target.NSQQueueDir,
			Description: queueDirComment,
			Optional:    true,
			Type:        "path",
		},
		config.HelpKV{
			Key:         target.NSQQueueLimit,
			Description: queueLimitComment,
			Optional:    true,
			Type:        "number",
		},
		config.HelpKV{
			Key:         config.Comment,
			Description: config.DefaultComment,
			Optional:    true,
			Type:        "sentence",
		},
	}

	HelpES = config.HelpKVS{
		config.HelpKV{
			Key:         target.ElasticURL,
			Description: "Elasticsearch server's address, with optional authentication info",
			Type:        "url",
		},
		config.HelpKV{
			Key:         target.ElasticIndex,
			Description: `Elasticsearch index to store/update events, index is auto-created`,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.ElasticFormat,
			Description: formatComment,
			Type:        "namespace*|access",
		},
		config.HelpKV{
			Key:         target.ElasticQueueDir,
			Description: queueDirComment,
			Optional:    true,
			Type:        "path",
		},
		config.HelpKV{
			Key:         target.ElasticQueueLimit,
			Description: queueLimitComment,
			Optional:    true,
			Type:        "number",
		},
		config.HelpKV{
			Key:         config.Comment,
			Description: config.DefaultComment,
			Optional:    true,
			Type:        "sentence",
		},
	}

	HelpRedis = config.HelpKVS{
		config.HelpKV{
			Key:         target.RedisAddress,
			Description: "Redis server's address. For example: `localhost:6379`",
			Type:        "address",
		},
		config.HelpKV{
			Key:         target.RedisKey,
			Description: "Redis key to store/update events, key is auto-created",
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.RedisFormat,
			Description: formatComment,
			Type:        "namespace*|access",
		},
		config.HelpKV{
			Key:         target.RedisPassword,
			Description: "Redis server password",
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.RedisQueueDir,
			Description: queueDirComment,
			Optional:    true,
			Type:        "path",
		},
		config.HelpKV{
			Key:         target.RedisQueueLimit,
			Description: queueLimitComment,
			Optional:    true,
			Type:        "number",
		},
		config.HelpKV{
			Key:         config.Comment,
			Description: config.DefaultComment,
			Optional:    true,
			Type:        "sentence",
		},
	}
)
