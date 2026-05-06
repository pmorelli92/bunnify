package bunnify

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type connectionOption struct {
	uri                    string
	reconnectInterval      time.Duration
	closeTimeout           time.Duration
	maxReconnectAttempts   int
	notificationChannel    chan<- Notification
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

// WithCloseTimeout sets the maximum time Close() will wait for the
// reconnectLoop goroutine and the in-flight conn.Close() to finish.
func WithCloseTimeout(d time.Duration) func(*connectionOption) {
	return func(opt *connectionOption) {
		opt.closeTimeout = d
	}
}

// WithMaxReconnectAttempts limits the total number of dial retries across
// all reconnect cycles. When n > 0 and the limit is reached,
// getNewChannel returns ErrMaxReconnectAttemptsReached and the loop exits.
func WithMaxReconnectAttempts(n int) func(*connectionOption) {
	return func(opt *connectionOption) {
		opt.maxReconnectAttempts = n
	}
}

// WithNotificationChannel specifies a go channel to receive messages
// such as connection established, reconnecting, event published, consumed, etc.
func WithNotificationChannel(notificationCh chan<- Notification) func(*connectionOption) {
	return func(opt *connectionOption) {
		opt.notificationChannel = notificationCh
	}
}

// connState is an immutable snapshot of the current AMQP connection and its
// readiness signal. ready is closed while conn is healthy; reconnectLoop installs
// a fresh connState (with a new open ready) each time the connection drops.
type connState struct {
	conn  *amqp.Connection
	ready chan struct{}
}

// Connection represents a connection towards the AMQP server.
// A single connection should be enough for the entire application as the
// consuming and publishing is handled by channels.
type Connection struct {
	options   connectionOption
	state     atomic.Pointer[connState]
	closeOnce sync.Once
	wg        sync.WaitGroup
	// done is closed once by Close() to permanently unblock all getNewChannel waiters.
	done chan struct{}
}

// NewConnection creates a new AMQP connection using the indicated
// options. If the consumer does not supply options, it will by default
// connect to a localhost instance on and try to reconnect every 10 seconds.
func NewConnection(opts ...func(*connectionOption)) *Connection {
	options := connectionOption{
		reconnectInterval: 10 * time.Second,
		closeTimeout:      5 * time.Second,
		uri:               "amqp://localhost:5672",
	}
	for _, opt := range opts {
		opt(&options)
	}
	c := &Connection{
		options: options,
		done:    make(chan struct{}),
	}
	initialReady := make(chan struct{})
	c.state.Store(&connState{ready: initialReady})
	return c
}

// Start establishes the connection towards the AMQP server.
// Only returns errors when the uri is not valid (retry won't do a thing)
func (c *Connection) Start() error {
	// Guard against calling Start() after Close().
	select {
	case <-c.done:
		return ErrConnectionClosedByUser
	default:
	}

	uri, err := amqp.ParseURI(c.options.uri)
	if err != nil {
		// Unblock any getNewChannel callers that were issued before Start returned.
		c.closeOnce.Do(func() { close(c.done) })
		return err
	}

	// Pass the initial ready channel so connect() owns exactly which channel to close.
	initialReady := c.state.Load().ready
	if err := c.connect(uri.String(), initialReady); err != nil {
		c.closeOnce.Do(func() { close(c.done) })
		return err
	}

	c.wg.Add(1)
	go c.reconnectLoop(uri.String())
	return nil
}

// connect dials the broker, retrying on failure until success or until the user
// calls Close. ready is the channel the caller owns and expects connect() to close
// once a live connection is stored — passing it explicitly avoids any risk of
// closing a stale or already-closed channel.
//
// Issue #7: the ticker is created only after the first failure so the very first
// dial attempt is immediate.
func (c *Connection) connect(uri string, ready chan struct{}) error {
	var ticker *time.Ticker
	var attempts int

	for {
		select {
		case <-c.done:
			return ErrConnectionClosedByUser
		default:
		}

		conn, err := amqp.Dial(uri)
		if err == nil {
			// Issue #16: notify before closing ready so observers see the notification
			// before getNewChannel unblocks.
			c.state.Store(&connState{conn: conn, ready: ready})
			notifyConnectionEstablished(c.options.notificationChannel)
			close(ready)

			// If Close() fired in the window between the last done-check and here,
			// close the just-dialled conn so it is not leaked.
			select {
			case <-c.done:
				conn.Close()
				return ErrConnectionClosedByUser
			default:
			}

			return nil
		}

		attempts++
		if c.options.maxReconnectAttempts > 0 && attempts >= c.options.maxReconnectAttempts {
			notifyConnectionFailed(c.options.notificationChannel, err)
			return ErrMaxReconnectAttemptsReached
		}

		notifyConnectionFailed(c.options.notificationChannel, err)

		// Create the ticker lazily so the very first retry is after one interval,
		// not penalising the initial dial attempt.
		if ticker == nil {
			ticker = time.NewTicker(c.options.reconnectInterval)
			defer ticker.Stop()
		}

		select {
		case <-c.done:
			return ErrConnectionClosedByUser
		case <-ticker.C:
		}
	}
}

// reconnectLoop runs in a single goroutine for the lifetime of the connection.
// It waits for the current connection to close and, unless the user requested
// shutdown, installs a fresh connState and dials a new connection — no
// recursion, no per-cycle goroutine.
func (c *Connection) reconnectLoop(uri string) {
	defer c.wg.Done()
	for {
		st := c.state.Load()
		<-st.conn.NotifyClose(make(chan *amqp.Error))

		// Create the new ready before storing so getNewChannel callers immediately
		// block on it rather than spinning on Channel() calls against the dead conn.
		newReady := make(chan struct{})
		c.state.Store(&connState{conn: st.conn, ready: newReady})

		select {
		case <-c.done:
			return
		default:
		}

		notifyConnectionLost(c.options.notificationChannel)
		if err := c.connect(uri, newReady); err != nil {
			// Only exit for expected terminal conditions; any other error keeps retrying.
			if errors.Is(err, ErrConnectionClosedByUser) || errors.Is(err, ErrMaxReconnectAttemptsReached) {
				return
			}
		}
	}
}

// Close shuts down the connection with towards the AMQP server
func (c *Connection) Close() error {
	c.closeOnce.Do(func() { close(c.done) })

	st := c.state.Load()
	var err error
	if st.conn != nil {
		notifyClosingConnection(c.options.notificationChannel)

		// Run conn.Close() with a timeout to avoid blocking forever on a wedged conn.
		done := make(chan error, 1)
		go func() { done <- st.conn.Close() }()

		timer := time.NewTimer(c.options.closeTimeout)
		defer timer.Stop()

		select {
		case closeErr := <-done:
			err = closeErr
		case <-timer.C:
		}

		// ErrClosed means connect()'s post-store guard already closed this conn; not an error.
		if errors.Is(err, amqp.ErrClosed) {
			err = nil
		}
	}

	// Wait for reconnectLoop to exit, but honour the close timeout.
	waitDone := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(waitDone)
	}()

	timer := time.NewTimer(c.options.closeTimeout)
	defer timer.Stop()

	select {
	case <-waitDone:
	case <-timer.C:
	}

	return err
}

func (c *Connection) getNewChannel(source NotificationSource) (*amqp.Channel, error) {
	for {
		st := c.state.Load()

		select {
		case <-c.done:
			return nil, ErrConnectionClosedByUser
		case <-st.ready:
		}

		// Re-read after ready: connect() stores the new conn before closing ready,
		// so this load is guaranteed to see the live connection (Go memory model:
		// close happens-before the receive that observes it).
		st = c.state.Load()

		if st.conn == nil {
			continue
		}

		ch, err := st.conn.Channel()
		if err == nil {
			// Issue #3/#4: guard against Close() tearing down the connection in the
			// window between Channel() succeeding and returning to the caller.
			select {
			case <-c.done:
				ch.Close()
				return nil, ErrConnectionClosedByUser
			default:
			}
			notifyChannelEstablished(c.options.notificationChannel, source)
			return ch, nil
		}
		notifyChannelFailed(c.options.notificationChannel, source, err)

		if !st.conn.IsClosed() {
			// Transient channel error (e.g. max channels reached). Back off before
			// retrying to avoid a tight spin.
			//
			// Issue #8/#9: use NewTimer instead of time.After to stop and drain
			// promptly when done fires.
			t := time.NewTimer(c.options.reconnectInterval)
			select {
			case <-c.done:
				t.Stop()
				return nil, ErrConnectionClosedByUser
			case <-t.C:
			}
		} else {
			// Issue #6: connection is closed; block until reconnectLoop installs a
			// fresh ready (or done fires) instead of spinning.
			freshSt := c.state.Load()
			select {
			case <-c.done:
				return nil, ErrConnectionClosedByUser
			case <-freshSt.ready:
			}
		}
	}
}
