package bunnify

import (
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type connectionOption struct {
	logger            Logger
	uri               string
	reconnectInterval time.Duration
}

func WithURI(URI string) func(*connectionOption) {
	return func(opt *connectionOption) {
		opt.uri = URI
	}
}

func WithReconnectInterval(interval time.Duration) func(*connectionOption) {
	return func(opt *connectionOption) {
		opt.reconnectInterval = interval
	}
}

func WithConnectionLogger(logger Logger) func(*connectionOption) {
	return func(opt *connectionOption) {
		opt.logger = logger
	}
}

type Connection struct {
	options                  connectionOption
	connection               *amqp.Connection
	connectionClosedBySystem bool
}

// NewConnection creates a new AMQP connection using the indicated
// options. If the consumer does not supply options, it will by default
// connect to a localhost instance, try to reconnect every 5 seconds
// and log connection related messages as json on the stdout.
func NewConnection(opts ...func(*connectionOption)) *Connection {
	options := connectionOption{
		reconnectInterval: 5 * time.Second,
		logger:            NewDefaultLogger(),
		uri:               "amqp://localhost:5672",
	}
	for _, opt := range opts {
		opt(&options)
	}
	return &Connection{
		options: options,
	}
}

// Start establishes the connection of the AMQP server.
func (c *Connection) Start() {
	var err error
	var conn *amqp.Connection
	ticker := time.NewTicker(c.options.reconnectInterval)

	for {
		conn, err = amqp.Dial(c.options.uri)
		if err == nil {
			break
		}

		c.options.logger.Error(fmt.Sprintf(serverConnectionFailed, err))
		<-ticker.C
	}

	c.options.logger.Info(serverConnectionEstablished)
	c.connection = conn

	go func() {
		<-conn.NotifyClose(make(chan *amqp.Error))
		if !c.connectionClosedBySystem {
			c.options.logger.Error(serverConnectionLost)
			c.Start()
		}
	}()
}

func (c *Connection) Close() error {
	c.connectionClosedBySystem = true
	if c.connection != nil {
		c.options.logger.Info(serverClosingConnection)
		return c.connection.Close()
	}
	return nil
}

func (c *Connection) getNewChannel() (*amqp.Channel, bool) {
	if c.connectionClosedBySystem {
		c.options.logger.Info(connectionClosedBySystem)
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

		c.options.logger.Error(fmt.Sprintf(channelConnectionFailed, err))
		<-ticker.C
	}

	c.options.logger.Info(channelConnectionEstablished)
	return ch, false
}
