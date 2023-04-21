package bunnify

import (
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type connectionOption struct {
	uri                 string
	reconnectInterval   time.Duration
	notificationChannel chan<- Notification
}

// WithURI allows the consumer to specify the AMQP Server.
// It should be in the format of amqp://0.0.0.0:5672
func WithURI(URI string) func(*connectionOption) {
	return func(opt *connectionOption) {
		opt.uri = URI
	}
}

// WithReconnectInterval establishes how much time to wait
// between each attempt of connection.
func WithReconnectInterval(interval time.Duration) func(*connectionOption) {
	return func(opt *connectionOption) {
		opt.reconnectInterval = interval
	}
}

// WithNotificationChannel specifies a go channel to receive messages
// such as connection established, reconnecting, event published, consumed, etc.
func WithNotificationChannel(notificationCh chan<- Notification) func(*connectionOption) {
	return func(opt *connectionOption) {
		opt.notificationChannel = notificationCh
	}
}

// Connection represents a connection towards the AMQP server.
// A single connection should be enough for the entire application as the
// consuming and publishing is handled by channels.
type Connection struct {
	options                  connectionOption
	connection               *amqp.Connection
	connectionClosedBySystem bool
}

// NewConnection creates a new AMQP connection using the indicated
// options. If the consumer does not supply options, it will by default
// connect to a localhost instance on, try to reconnect every 5 seconds
// and log connection related messages as json on the stdout.
func NewConnection(opts ...func(*connectionOption)) *Connection {
	options := connectionOption{
		reconnectInterval: 5 * time.Second,
		uri:               "amqp://localhost:5672",
	}
	for _, opt := range opts {
		opt(&options)
	}
	return &Connection{
		options: options,
	}
}

// Start establishes the connection towards the AMQP server.
func (c *Connection) Start() {
	var err error
	var conn *amqp.Connection
	ticker := time.NewTicker(c.options.reconnectInterval)

	for {
		conn, err = amqp.Dial(c.options.uri)
		if err == nil {
			break
		}

		sendError(
			c.options.notificationChannel,
			NotificationProducerConnection,
			fmt.Sprintf(serverConnectionFailed, err))

		<-ticker.C
	}

	c.connection = conn
	sendInfo(
		c.options.notificationChannel,
		NotificationProducerConnection,
		serverConnectionEstablished)

	go func() {
		<-conn.NotifyClose(make(chan *amqp.Error))
		if !c.connectionClosedBySystem {
			sendError(
				c.options.notificationChannel,
				NotificationProducerConnection,
				serverConnectionLost)

			c.Start()
		}
	}()
}

// Closes connection with towards the AMQP server
func (c *Connection) Close() error {
	c.connectionClosedBySystem = true
	if c.connection != nil {
		sendInfo(
			c.options.notificationChannel,
			NotificationProducerConnection,
			serverClosingConnection)

		return c.connection.Close()
	}
	return nil
}

func (c *Connection) getNewChannel(producer NotificationProducer) (*amqp.Channel, bool) {
	if c.connectionClosedBySystem {
		sendInfo(
			c.options.notificationChannel,
			producer,
			connectionClosedBySystem)
		return nil, true
	}

	var err error
	var ch *amqp.Channel
	ticker := time.NewTicker(c.options.reconnectInterval)

	for {
		ch, err = c.connection.Channel()
		if err == nil {
			break
		}

		sendError(
			c.options.notificationChannel,
			producer,
			fmt.Sprintf(channelConnectionFailed, err))

		<-ticker.C
	}

	sendInfo(
		c.options.notificationChannel,
		producer,
		channelConnectionEstablished)

	return ch, false
}
