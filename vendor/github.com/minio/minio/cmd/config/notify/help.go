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

// Help template inputs for all notification targets
var (
	HelpAMQP = config.HelpKVS{
		config.HelpKV{
			Key:         target.AmqpURL,
<<<<<<< HEAD
			Description: "AMQP server endpoint e.g. `amqp://myuser:mypassword@localhost:5672`",
=======
			Description: "AMQP server endpoint, e.g. `amqp://myuser:mypassword@localhost:5672`",
>>>>>>> 524_bug
			Type:        "url",
		},
		config.HelpKV{
			Key:         target.AmqpExchange,
<<<<<<< HEAD
			Description: "name of the AMQP exchange",
=======
			Description: "Name of the AMQP exchange",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.AmqpExchangeType,
<<<<<<< HEAD
			Description: "kind of AMQP exchange type",
=======
			Description: "Kind of AMQP exchange type",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.AmqpRoutingKey,
<<<<<<< HEAD
			Description: "routing key for publishing",
=======
			Description: "Routing key for publishing",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.AmqpMandatory,
<<<<<<< HEAD
			Description: "set this to 'on' for server to return an unroutable message with a Return method. If this flag is 'off', the server silently drops the message",
=======
			Description: "Set this to 'on' for server to return an unroutable message with a Return method. If this flag is 'off', the server silently drops the message",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "on|off",
		},
		config.HelpKV{
			Key:         target.AmqpDurable,
<<<<<<< HEAD
			Description: "set this to 'on' for queue to survive broker restarts",
=======
			Description: "Set this to 'on' for queue to survive broker restarts",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "on|off",
		},
		config.HelpKV{
			Key:         target.AmqpNoWait,
<<<<<<< HEAD
			Description: "when no_wait is 'on', declare without waiting for a confirmation from the server",
=======
			Description: "When no_wait is 'on', declare without waiting for a confirmation from the server",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "on|off",
		},
		config.HelpKV{
			Key:         target.AmqpInternal,
<<<<<<< HEAD
			Description: "set this to 'on' for exchange to be not used directly by publishers, but only when bound to other exchanges",
=======
			Description: "Set this to 'on' for exchange to be not used directly by publishers, but only when bound to other exchanges",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "on|off",
		},
		config.HelpKV{
			Key:         target.AmqpAutoDeleted,
<<<<<<< HEAD
			Description: "set this to 'on' for queue that has had at least one consumer is deleted when last consumer unsubscribes",
=======
			Description: "Set this to 'on' for queue that has had at least one consumer is deleted when last consumer unsubscribes",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "on|off",
		},
		config.HelpKV{
			Key:         target.AmqpDeliveryMode,
<<<<<<< HEAD
			Description: "delivery queue implementation use non-persistent (1) or persistent (2)",
=======
			Description: "Delivery queue implementation use non-persistent (1) or persistent (2)",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "number",
		},
		config.HelpKV{
			Key:         target.AmqpQueueDir,
<<<<<<< HEAD
			Description: "local directory where events are stored e.g. '/home/events'",
=======
			Description: "Local directory where events are stored eg: '/home/events'",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "path",
		},
		config.HelpKV{
			Key:         target.AmqpQueueLimit,
<<<<<<< HEAD
			Description: "enable persistent event store queue limit, defaults to '10000'",
=======
			Description: "Enable persistent event store queue limit, defaults to '10000'",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "number",
		},
		config.HelpKV{
			Key:         config.Comment,
<<<<<<< HEAD
			Description: config.DefaultComment,
=======
			Description: "A comment to describe the AMQP target setting",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "sentence",
		},
	}

	HelpKafka = config.HelpKVS{
		config.HelpKV{
			Key:         target.KafkaBrokers,
<<<<<<< HEAD
			Description: "comma separated list of Kafka broker addresses",
=======
			Description: "Comma separated list of Kafka broker addresses",
>>>>>>> 524_bug
			Type:        "csv",
		},
		config.HelpKV{
			Key:         target.KafkaTopic,
<<<<<<< HEAD
			Description: "Kafka topic used for bucket notifications",
=======
			Description: "The Kafka topic for a given message",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.KafkaSASLUsername,
<<<<<<< HEAD
			Description: "username for SASL/PLAIN or SASL/SCRAM authentication",
=======
			Description: "Username for SASL/PLAIN  or SASL/SCRAM authentication",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.KafkaSASLPassword,
<<<<<<< HEAD
			Description: "password for SASL/PLAIN or SASL/SCRAM authentication",
=======
			Description: "Password for SASL/PLAIN  or SASL/SCRAM authentication",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.KafkaTLSClientAuth,
<<<<<<< HEAD
			Description: "clientAuth determines the Kafka server's policy for TLS client auth",
=======
			Description: "ClientAuth determines the Kafka server's policy for TLS client auth",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.KafkaSASL,
<<<<<<< HEAD
			Description: "set this to 'on' to enable SASL authentication",
=======
			Description: "Set this to 'on' to enable SASL authentication",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "on|off",
		},
		config.HelpKV{
			Key:         target.KafkaTLS,
<<<<<<< HEAD
			Description: "set this to 'on' to enable TLS",
=======
			Description: "Set this to 'on' to enable TLS",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "on|off",
		},
		config.HelpKV{
			Key:         target.KafkaTLSSkipVerify,
<<<<<<< HEAD
			Description: "set this to 'on' to disable client verification of server certificate chain",
=======
			Description: "Set this to 'on' to disable client verification of server certificate chain",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "on|off",
		},
		config.HelpKV{
			Key:         target.KafkaQueueDir,
<<<<<<< HEAD
			Description: "local directory where events are stored e.g. '/home/events'",
=======
			Description: "Local directory where events are stored eg: '/home/events'",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "path",
		},
		config.HelpKV{
			Key:         target.KafkaQueueLimit,
<<<<<<< HEAD
			Description: "enable persistent event store queue limit, defaults to '10000'",
=======
			Description: "Enable persistent event store queue limit, defaults to '10000'",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "number",
		},
		config.HelpKV{
<<<<<<< HEAD
			Key:         target.KafkaClientTLSCert,
			Description: "Set path to client certificate",
			Optional:    true,
			Type:        "path",
		},
		config.HelpKV{
			Key:         target.KafkaClientTLSKey,
			Description: "Set path to client key",
			Optional:    true,
			Type:        "path",
		},
		config.HelpKV{
			Key:         config.Comment,
			Description: config.DefaultComment,
=======
			Key:         config.Comment,
			Description: "A comment to describe the Kafka target setting",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "sentence",
		},
	}

	HelpMQTT = config.HelpKVS{
		config.HelpKV{
			Key:         target.MqttBroker,
<<<<<<< HEAD
			Description: "MQTT server endpoint e.g. `tcp://localhost:1883`",
=======
			Description: "MQTT server endpoint, e.g. `tcp://localhost:1883`",
>>>>>>> 524_bug
			Type:        "uri",
		},
		config.HelpKV{
			Key:         target.MqttTopic,
<<<<<<< HEAD
			Description: "name of the MQTT topic to publish on, e.g. `minio`",
=======
			Description: "Name of the MQTT topic to publish on, e.g. `minio`",
>>>>>>> 524_bug
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.MqttUsername,
<<<<<<< HEAD
			Description: "username to connect to the MQTT server",
=======
			Description: "Username to connect to the MQTT server",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.MqttPassword,
<<<<<<< HEAD
			Description: "password to connect to the MQTT server",
=======
			Description: "Password to connect to the MQTT server",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.MqttQoS,
<<<<<<< HEAD
			Description: "set the Quality of Service Level for MQTT endpoint",
=======
			Description: "Set the Quality of Service Level for MQTT endpoint",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "number",
		},
		config.HelpKV{
			Key:         target.MqttKeepAliveInterval,
<<<<<<< HEAD
			Description: "keep alive interval for MQTT connections",
=======
			Description: "Keep alive interval for MQTT connections",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "duration",
		},
		config.HelpKV{
			Key:         target.MqttReconnectInterval,
<<<<<<< HEAD
			Description: "reconnect interval for MQTT connections",
=======
			Description: "Reconnect interval for MQTT connections",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "duration",
		},
		config.HelpKV{
			Key:         target.MqttQueueDir,
<<<<<<< HEAD
			Description: "local directory where events are stored e.g. '/home/events'",
=======
			Description: "Local directory where events are stored eg: '/home/events'",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "path",
		},
		config.HelpKV{
			Key:         target.MqttQueueLimit,
<<<<<<< HEAD
			Description: "enable persistent event store queue limit, defaults to '10000'",
=======
			Description: "Enable persistent event store queue limit, defaults to '10000'",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "number",
		},
		config.HelpKV{
			Key:         config.Comment,
<<<<<<< HEAD
			Description: config.DefaultComment,
=======
			Description: "A comment to describe the MQTT target setting",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "sentence",
		},
	}

	HelpES = config.HelpKVS{
		config.HelpKV{
			Key:         target.ElasticURL,
<<<<<<< HEAD
			Description: "Elasticsearch server's address, with optional authentication info",
=======
			Description: "The Elasticsearch server's address, with optional authentication info",
>>>>>>> 524_bug
			Type:        "url",
		},
		config.HelpKV{
			Key:         target.ElasticFormat,
<<<<<<< HEAD
			Description: "set this to `namespace` or `access`, defaults to 'namespace'",
=======
			Description: "Either `namespace` or `access`, defaults to 'namespace'",
>>>>>>> 524_bug
			Type:        "namespace*|access",
		},
		config.HelpKV{
			Key:         target.ElasticIndex,
<<<<<<< HEAD
			Description: "the name of an Elasticsearch index in which MinIO will store document",
=======
			Description: "The name of an Elasticsearch index in which MinIO will store document",
>>>>>>> 524_bug
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.ElasticQueueDir,
<<<<<<< HEAD
			Description: "local directory where events are stored e.g. '/home/events'",
=======
			Description: "Local directory where events are stored eg: '/home/events'",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "path",
		},
		config.HelpKV{
			Key:         target.ElasticQueueLimit,
<<<<<<< HEAD
			Description: "enable persistent event store queue limit, defaults to '10000'",
=======
			Description: "Enable persistent event store queue limit, defaults to '10000'",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "number",
		},
		config.HelpKV{
			Key:         config.Comment,
<<<<<<< HEAD
			Description: config.DefaultComment,
=======
			Description: "A comment to describe the Elasticsearch target setting",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "sentence",
		},
	}

	HelpWebhook = config.HelpKVS{
		config.HelpKV{
			Key:         target.WebhookEndpoint,
<<<<<<< HEAD
			Description: "webhook server endpoint e.g. http://localhost:8080/minio/events",
=======
			Description: "Webhook server endpoint eg: http://localhost:8080/minio/events",
>>>>>>> 524_bug
			Type:        "url",
		},
		config.HelpKV{
			Key:         target.WebhookAuthToken,
<<<<<<< HEAD
			Description: "authorization token used for webhook server endpoint",
=======
			Description: "Authorization token used for webhook server endpoint",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.WebhookQueueDir,
<<<<<<< HEAD
			Description: "local directory where events are stored e.g. '/home/events'",
=======
			Description: "Local directory where events are stored eg: '/home/events'",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "path",
		},
		config.HelpKV{
			Key:         target.WebhookQueueLimit,
<<<<<<< HEAD
			Description: "enable persistent event store queue limit, defaults to '10000'",
=======
			Description: "Enable persistent event store queue limit, defaults to '10000'",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "number",
		},
		config.HelpKV{
			Key:         config.Comment,
<<<<<<< HEAD
			Description: config.DefaultComment,
=======
			Description: "A comment to describe the Webhook target setting",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "sentence",
		},
	}

	HelpRedis = config.HelpKVS{
		config.HelpKV{
			Key:         target.RedisAddress,
<<<<<<< HEAD
			Description: "Redis server's address. For example: `localhost:6379`",
=======
			Description: "The Redis server's address. For example: `localhost:6379`",
>>>>>>> 524_bug
			Type:        "address",
		},
		config.HelpKV{
			Key:         target.RedisFormat,
<<<<<<< HEAD
			Description: "specifies how data is populated, a hash is used in case of `namespace` format and a list in case of `access` format, defaults to 'namespace'",
			Type:        "namespace*|access",
		},
		config.HelpKV{
			Key:         target.RedisKey,
			Description: "name of the Redis key under which events are stored",
=======
			Description: "Specify how data is populated, a hash is used in case of `namespace` format and a list in case of `access` format, defaults to 'namespace'",
			Type:        "namespace|access",
		},
		config.HelpKV{
			Key:         target.RedisKey,
			Description: "The name of the Redis key under which events are stored",
>>>>>>> 524_bug
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.RedisPassword,
<<<<<<< HEAD
			Description: "Redis server's password",
=======
			Description: "The Redis server's password",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.RedisQueueDir,
<<<<<<< HEAD
			Description: "local directory where events are stored e.g. '/home/events'",
=======
			Description: "Local directory where events are stored eg: '/home/events'",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "path",
		},
		config.HelpKV{
			Key:         target.RedisQueueLimit,
<<<<<<< HEAD
			Description: "enable persistent event store queue limit, defaults to '10000'",
=======
			Description: "Enable persistent event store queue limit, defaults to '10000'",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "number",
		},
		config.HelpKV{
			Key:         config.Comment,
<<<<<<< HEAD
			Description: config.DefaultComment,
=======
			Description: "A comment to describe the Redis target setting",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "sentence",
		},
	}

	HelpPostgres = config.HelpKVS{
		config.HelpKV{
			Key:         target.PostgresConnectionString,
<<<<<<< HEAD
			Description: "connection string parameters for the PostgreSQL server",
=======
			Description: "Connection string parameters for the PostgreSQL server",
>>>>>>> 524_bug
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.PostgresFormat,
<<<<<<< HEAD
			Description: "specifies how data is populated, `namespace` format and `access` format, defaults to 'namespace'",
			Type:        "namespace*|access",
		},
		config.HelpKV{
			Key:         target.PostgresTable,
			Description: "table name in which events will be stored/updated. If the table does not exist, the MinIO server creates it at start-up",
=======
			Description: "Specify how data is populated, `namespace` format and `access` format, defaults to 'namespace'",
			Type:        "namespace|access",
		},
		config.HelpKV{
			Key:         target.PostgresTable,
			Description: "Table name in which events will be stored/updated. If the table does not exist, the MinIO server creates it at start-up",
>>>>>>> 524_bug
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.PostgresHost,
<<<<<<< HEAD
			Description: "host name of the PostgreSQL server. Defaults to `localhost`. IPv6 host should be enclosed with `[` and `]`",
=======
			Description: "Host name of the PostgreSQL server. Defaults to `localhost`. IPv6 host should be enclosed with `[` and `]`",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "hostname",
		},
		config.HelpKV{
			Key:         target.PostgresPort,
<<<<<<< HEAD
			Description: "port on which to connect to PostgreSQL server, defaults to `5432`",
=======
			Description: "Port on which to connect to PostgreSQL server, defaults to `5432`",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "port",
		},
		config.HelpKV{
			Key:         target.PostgresUsername,
<<<<<<< HEAD
			Description: "database username, defaults to user running the MinIO process if not specified",
=======
			Description: "Database username, defaults to user running the MinIO process if not specified",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.PostgresPassword,
<<<<<<< HEAD
			Description: "database password",
=======
			Description: "Database password",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.PostgresDatabase,
<<<<<<< HEAD
			Description: "postgres Database name",
=======
			Description: "Postgres Database name",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.PostgresQueueDir,
<<<<<<< HEAD
			Description: "local directory where events are stored e.g. '/home/events'",
=======
			Description: "Local directory where events are stored eg: '/home/events'",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "path",
		},
		config.HelpKV{
			Key:         target.PostgresQueueLimit,
<<<<<<< HEAD
			Description: "enable persistent event store queue limit, defaults to '10000'",
=======
			Description: "Enable persistent event store queue limit, defaults to '10000'",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "number",
		},
		config.HelpKV{
			Key:         config.Comment,
<<<<<<< HEAD
			Description: config.DefaultComment,
=======
			Description: "A comment to describe the Postgres target setting",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "sentence",
		},
	}

	HelpMySQL = config.HelpKVS{
		config.HelpKV{
			Key:         target.MySQLDSNString,
<<<<<<< HEAD
			Description: "data source name connection string for the MySQL server",
=======
			Description: "Data-Source-Name connection string for the MySQL server",
>>>>>>> 524_bug
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.MySQLTable,
<<<<<<< HEAD
			Description: "table name in which events will be stored/updated. If the table does not exist, the MinIO server creates it at start-up",
=======
			Description: "Table name in which events will be stored/updated. If the table does not exist, the MinIO server creates it at start-up",
>>>>>>> 524_bug
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.MySQLFormat,
<<<<<<< HEAD
			Description: "specifies how data is populated, `namespace` format and `access` format, defaults to 'namespace'",
			Type:        "namespace*|access",
		},
		config.HelpKV{
			Key:         target.MySQLHost,
			Description: "host name of the MySQL server (used only if `dsnString` is empty)",
=======
			Description: "Specify how data is populated, `namespace` format and `access` format, defaults to 'namespace'",
			Type:        "namespace|access",
		},
		config.HelpKV{
			Key:         target.MySQLHost,
			Description: "Host name of the MySQL server (used only if `dsnString` is empty)",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "hostname",
		},
		config.HelpKV{
			Key:         target.MySQLPort,
<<<<<<< HEAD
			Description: "port on which to connect to the MySQL server (used only if `dsn_string` is empty)",
=======
			Description: "Port on which to connect to the MySQL server (used only if `dsn_string` is empty)",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "port",
		},
		config.HelpKV{
			Key:         target.MySQLUsername,
<<<<<<< HEAD
			Description: "database user-name (used only if `dsnString` is empty)",
=======
			Description: "Database user-name (used only if `dsnString` is empty)",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.MySQLPassword,
<<<<<<< HEAD
			Description: "database password (used only if `dsnString` is empty)",
=======
			Description: "Database password (used only if `dsnString` is empty)",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.MySQLDatabase,
<<<<<<< HEAD
			Description: "database name (used only if `dsnString` is empty)",
=======
			Description: "Database name (used only if `dsnString` is empty)",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.MySQLQueueDir,
<<<<<<< HEAD
			Description: "local directory where events are stored e.g. '/home/events'",
=======
			Description: "Local directory where events are stored eg: '/home/events'",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "path",
		},
		config.HelpKV{
			Key:         target.MySQLQueueLimit,
<<<<<<< HEAD
			Description: "enable persistent event store queue limit, defaults to '10000'",
=======
			Description: "Enable persistent event store queue limit, defaults to '10000'",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "number",
		},
		config.HelpKV{
			Key:         config.Comment,
<<<<<<< HEAD
			Description: config.DefaultComment,
=======
			Description: "A comment to describe the MySQL target setting",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "sentence",
		},
	}

	HelpNATS = config.HelpKVS{
		config.HelpKV{
			Key:         target.NATSAddress,
<<<<<<< HEAD
			Description: "NATS server address e.g. '0.0.0.0:4222'",
=======
			Description: "NATS server address eg: '0.0.0.0:4222'",
>>>>>>> 524_bug
			Type:        "address",
		},
		config.HelpKV{
			Key:         target.NATSSubject,
			Description: "NATS subject that represents this subscription",
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.NATSUsername,
<<<<<<< HEAD
			Description: "username to be used when connecting to the server",
=======
			Description: "Username to be used when connecting to the server",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.NATSPassword,
<<<<<<< HEAD
			Description: "password to be used when connecting to a server",
=======
			Description: "Password to be used when connecting to a server",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.NATSToken,
<<<<<<< HEAD
			Description: "token to be used when connecting to a server",
=======
			Description: "Token to be used when connecting to a server",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.NATSSecure,
<<<<<<< HEAD
			Description: "set this to 'on', enables TLS secure connections that skip server verification (not recommended)",
=======
			Description: "Set this to 'on', enables TLS secure connections that skip server verification (not recommended)",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "on|off",
		},
		config.HelpKV{
			Key:         target.NATSPingInterval,
<<<<<<< HEAD
			Description: "client ping commands interval to the server, disabled by default",
=======
			Description: "Client ping commands interval to the server, disabled by default",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "duration",
		},
		config.HelpKV{
			Key:         target.NATSStreaming,
<<<<<<< HEAD
			Description: "set this to 'on', to use streaming NATS server",
=======
			Description: "Set this to 'on', to use streaming NATS server",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "on|off",
		},
		config.HelpKV{
			Key:         target.NATSStreamingAsync,
<<<<<<< HEAD
			Description: "set this to 'on', to enable asynchronous publish, process the ACK or error state",
=======
			Description: "Set this to 'on', to enable asynchronous publish, process the ACK or error state",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "on|off",
		},
		config.HelpKV{
			Key:         target.NATSStreamingMaxPubAcksInFlight,
<<<<<<< HEAD
			Description: "specifies how many messages can be published without getting ACKs back from NATS streaming server",
=======
			Description: "Specifies how many messages can be published without getting ACKs back from NATS streaming server",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "number",
		},
		config.HelpKV{
			Key:         target.NATSStreamingClusterID,
<<<<<<< HEAD
			Description: "unique ID for the NATS streaming cluster",
=======
			Description: "Unique ID for the NATS streaming cluster",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.NATSQueueLimit,
<<<<<<< HEAD
			Description: "enable persistent event store queue limit, defaults to '10000'",
=======
			Description: "Enable persistent event store queue limit, defaults to '10000'",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "number",
		},
		config.HelpKV{
			Key:         target.NATSQueueDir,
<<<<<<< HEAD
			Description: "local directory where events are stored e.g. '/home/events'",
=======
			Description: "Local directory where events are stored eg: '/home/events'",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "path",
		},
		config.HelpKV{
			Key:         target.NATSCertAuthority,
<<<<<<< HEAD
			Description: "certificate chain of the target NATS server if self signed certs were used",
=======
			Description: "Certificate chain of the target NATS server if self signed certs were used",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.NATSClientCert,
<<<<<<< HEAD
			Description: "TLS Cert used for NATS configured to require client certificates",
=======
			Description: "TLS Cert used to authenticate against NATS configured to require client certificates",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.NATSClientKey,
<<<<<<< HEAD
			Description: "TLS Key used for NATS configured to require client certificates",
=======
			Description: "TLS Key used to authenticate against NATS configured to require client certificates",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         config.Comment,
<<<<<<< HEAD
			Description: config.DefaultComment,
=======
			Description: "A comment to describe the NATS target setting",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "sentence",
		},
	}

	HelpNSQ = config.HelpKVS{
		config.HelpKV{
			Key:         target.NSQAddress,
<<<<<<< HEAD
			Description: "NSQ server address e.g. '127.0.0.1:4150'",
=======
			Description: "NSQ server address eg: '127.0.0.1:4150'",
>>>>>>> 524_bug
			Type:        "address",
		},
		config.HelpKV{
			Key:         target.NSQTopic,
			Description: "NSQ topic unique per target",
			Type:        "string",
		},
		config.HelpKV{
			Key:         target.NSQTLS,
<<<<<<< HEAD
			Description: "set this to 'on', to enable TLS negotiation",
=======
			Description: "Set this to 'on', to enable TLS negotiation",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "on|off",
		},
		config.HelpKV{
			Key:         target.NSQTLSSkipVerify,
<<<<<<< HEAD
			Description: "set this to 'on', to disable client verification of server certificates",
=======
			Description: "Set this to 'on', to disable client verification of server certificates",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "on|off",
		},
		config.HelpKV{
			Key:         target.NSQQueueDir,
<<<<<<< HEAD
			Description: "local directory where events are stored e.g. '/home/events'",
=======
			Description: "Local directory where events are stored eg: '/home/events'",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "path",
		},
		config.HelpKV{
			Key:         target.NSQQueueLimit,
<<<<<<< HEAD
			Description: "enable persistent event store queue limit, defaults to '10000'",
=======
			Description: "Enable persistent event store queue limit, defaults to '10000'",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "number",
		},
		config.HelpKV{
			Key:         config.Comment,
<<<<<<< HEAD
			Description: config.DefaultComment,
=======
			Description: "A comment to describe the NSQ target setting",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "sentence",
		},
	}
)
