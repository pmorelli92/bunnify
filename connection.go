package bunnify

import (
	"sync"
	"sync/atomic"
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
	options                connectionOption
	mu                     sync.RWMutex
	connection             *amqp.Connection
	connectionClosedByUser atomic.Bool
}

// NewConnection creates a new AMQP connection using the indicated
// options. If the consumer does not supply options, it will by default
// connect to a localhost instance on and try to reconnect every 10 seconds.
func NewConnection(opts ...func(*connectionOption)) *Connection {
	options := connectionOption{
		reconnectInterval: 10 * time.Second,
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
// Only returns errors when the uri is not valid (retry won't do a thing)
func (c *Connection) Start() error {
	uri, err := amqp.ParseURI(c.options.uri)
	if err != nil {
		return err
	}

	if err := c.connect(uri.String()); err != nil {
		return err
	}

	go c.reconnectLoop(uri.String())
	return nil
}

// connect dials the broker, retrying on failure until success or until the
// user calls Close. On success the new connection is published under the lock.
func (c *Connection) connect(uri string) error {
	ticker := time.NewTicker(c.options.reconnectInterval)
	defer ticker.Stop()

	var conn *amqp.Connection
	var err error
	for {
		if c.connectionClosedByUser.Load() {
			return errConnectionClosedByUser
		}

		conn, err = amqp.Dial(uri)
		if err == nil {
			break
		}

		notifyConnectionFailed(c.options.notificationChannel, err)
		<-ticker.C
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connectionClosedByUser.Load() {
		return errConnectionClosedByUser
	}
	c.connection = conn

	notifyConnectionEstablished(c.options.notificationChannel)
	return nil
}

// reconnectLoop runs in a single goroutine for the lifetime of the connection.
// It waits for the current connection to close and, unless the user requested
// shutdown, dials a new one in place — no recursion, no per-cycle goroutine.
func (c *Connection) reconnectLoop(uri string) {
	for {
		c.mu.RLock()
		conn := c.connection
		c.mu.RUnlock()

		<-conn.NotifyClose(make(chan *amqp.Error))

		if c.connectionClosedByUser.Load() {
			return
		}

		notifyConnectionLost(c.options.notificationChannel)
		if err := c.connect(uri); err != nil {
			return
		}
	}
}

// Closes connection with towards the AMQP server
func (c *Connection) Close() error {
	c.connectionClosedByUser.Store(true)

	c.mu.RLock()
	conn := c.connection
	c.mu.RUnlock()

	if conn != nil {
		notifyClosingConnection(c.options.notificationChannel)
		return conn.Close()
	}
	return nil
}

func (c *Connection) getNewChannel(source NotificationSource) (*amqp.Channel, bool) {
	if c.connectionClosedByUser.Load() {
		return nil, true
	}

	ticker := time.NewTicker(c.options.reconnectInterval)
	defer ticker.Stop()

	for {
		if c.connectionClosedByUser.Load() {
			return nil, true
		}

		c.mu.RLock()
		conn := c.connection
		c.mu.RUnlock()

		ch, err := conn.Channel()
		if err == nil {
			notifyChannelEstablished(c.options.notificationChannel, source)
			return ch, false
		}

		notifyChannelFailed(c.options.notificationChannel, source, err)
		<-ticker.C
	}
}
