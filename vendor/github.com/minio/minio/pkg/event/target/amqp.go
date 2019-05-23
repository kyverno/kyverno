/*
 * MinIO Cloud Storage, (C) 2018 MinIO, Inc.
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

package target

import (
	"encoding/json"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/minio/minio/pkg/event"
	xnet "github.com/minio/minio/pkg/net"
	"github.com/streadway/amqp"
)

// AMQPArgs - AMQP target arguments.
type AMQPArgs struct {
	Enable       bool     `json:"enable"`
	URL          xnet.URL `json:"url"`
	Exchange     string   `json:"exchange"`
	RoutingKey   string   `json:"routingKey"`
	ExchangeType string   `json:"exchangeType"`
	DeliveryMode uint8    `json:"deliveryMode"`
	Mandatory    bool     `json:"mandatory"`
	Immediate    bool     `json:"immediate"`
	Durable      bool     `json:"durable"`
	Internal     bool     `json:"internal"`
	NoWait       bool     `json:"noWait"`
	AutoDeleted  bool     `json:"autoDeleted"`
}

// Validate AMQP arguments
func (a *AMQPArgs) Validate() error {
	if !a.Enable {
		return nil
	}
	if _, err := amqp.ParseURI(a.URL.String()); err != nil {
		return err
	}
	return nil
}

// AMQPTarget - AMQP target
type AMQPTarget struct {
	id        event.TargetID
	args      AMQPArgs
	conn      *amqp.Connection
	connMutex sync.Mutex
}

// ID - returns TargetID.
func (target *AMQPTarget) ID() event.TargetID {
	return target.id
}

func (target *AMQPTarget) channel() (*amqp.Channel, error) {
	isAMQPClosedErr := func(err error) bool {
		if err == amqp.ErrClosed {
			return true
		}

		if nerr, ok := err.(*net.OpError); ok {
			return (nerr.Err.Error() == "use of closed network connection")
		}

		return false
	}

	target.connMutex.Lock()
	defer target.connMutex.Unlock()

	ch, err := target.conn.Channel()
	if err == nil {
		return ch, nil
	}

	if !isAMQPClosedErr(err) {
		return nil, err
	}

	var conn *amqp.Connection
	if conn, err = amqp.Dial(target.args.URL.String()); err != nil {
		return nil, err
	}

	if ch, err = conn.Channel(); err != nil {
		return nil, err
	}

	target.conn = conn

	return ch, nil
}

// Save - Sends event directly without persisting.
func (target *AMQPTarget) Save(eventData event.Event) error {
	return target.send(eventData)
}

func (target *AMQPTarget) send(eventData event.Event) error {
	ch, err := target.channel()
	if err != nil {
		return err
	}
	defer func() {
		// FIXME: log returned error. ignore time being.
		_ = ch.Close()
	}()

	objectName, err := url.QueryUnescape(eventData.S3.Object.Key)
	if err != nil {
		return err
	}
	key := eventData.S3.Bucket.Name + "/" + objectName

	data, err := json.Marshal(event.Log{EventName: eventData.EventName, Key: key, Records: []event.Event{eventData}})
	if err != nil {
		return err
	}

	if err = ch.ExchangeDeclare(target.args.Exchange, target.args.ExchangeType, target.args.Durable,
		target.args.AutoDeleted, target.args.Internal, target.args.NoWait, nil); err != nil {
		return err
	}

	return ch.Publish(target.args.Exchange, target.args.RoutingKey, target.args.Mandatory,
		target.args.Immediate, amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: target.args.DeliveryMode,
			Body:         data,
		})
}

// Send - interface compatible method does no-op.
func (target *AMQPTarget) Send(eventKey string) error {
	return nil
}

// Close - does nothing and available for interface compatibility.
func (target *AMQPTarget) Close() error {
	return nil
}

// NewAMQPTarget - creates new AMQP target.
func NewAMQPTarget(id string, args AMQPArgs) (*AMQPTarget, error) {
	var conn *amqp.Connection
	var err error
	// Retry 5 times with time interval of 2 seconds.
	for i := 1; i <= 5; i++ {
		conn, err = amqp.Dial(args.URL.String())
		if err == nil {
			break
		}
		if err != nil && i == 5 {
			return nil, err
		}
		time.Sleep(2 * time.Second)
	}

	return &AMQPTarget{
		id:   event.TargetID{ID: id, Name: "amqp"},
		args: args,
		conn: conn,
	}, nil
}
