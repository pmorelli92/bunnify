package bunnify

import (
	"errors"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type connectionOption struct {
	uri                  string
	reconnectInterval    time.Duration
	channelRetryInterval time.Duration
	closeTimeout         time.Duration
	dialTimeout          time.Duration
	heartbeat            time.Duration
	maxReconnectAttempts int
	notificationChannel  chan<- Notification
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

// WithChannelRetryInterval sets how long getNewChannel waits before retrying
// after a transient channel creation failure (e.g. max channels reached,
// server-side throttle) while the connection is still alive.
// Defaults to 500ms.
func WithChannelRetryInterval(d time.Duration) func(*connectionOption) {
	return func(opt *connectionOption) {
		opt.channelRetryInterval = d
	}
}

// WithCloseTimeout sets the maximum time Close() will wait for the
// reconnectLoop goroutine and the in-flight conn.Close() to finish.
func WithCloseTimeout(d time.Duration) func(*connectionOption) {
	return func(opt *connectionOption) {
		opt.closeTimeout = d
	}
}

// WithMaxReconnectAttempts limits the number of consecutive dial retries during a
// single outage. When n > 0 and n consecutive attempts fail, connect returns
// ErrMaxReconnectAttemptsReached and the reconnect loop exits. The counter resets
// to zero after each successful connection, so a connection that flaps repeatedly
// does not accumulate toward the limit across separate outage windows.
func WithMaxReconnectAttempts(n int) func(*connectionOption) {
	return func(opt *connectionOption) {
		opt.maxReconnectAttempts = n
	}
}

// WithDialTimeout sets the maximum time allowed for a single TCP dial attempt
// to the broker. If the dial does not complete within this duration the attempt
// is aborted and the reconnect loop retries. Defaults to 5s.
func WithDialTimeout(d time.Duration) func(*connectionOption) {
	return func(opt *connectionOption) {
		opt.dialTimeout = d
	}
}

// WithHeartbeat sets the AMQP heartbeat interval negotiated with the broker.
// A dead network is detected within approximately 2× this value. Defaults to 5s.
func WithHeartbeat(d time.Duration) func(*connectionOption) {
	return func(opt *connectionOption) {
		opt.heartbeat = d
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
		reconnectInterval:    10 * time.Second,
		channelRetryInterval: 500 * time.Millisecond,
		closeTimeout:         5 * time.Second,
		dialTimeout:          5 * time.Second,
		heartbeat:            5 * time.Second,
		uri:                  "amqp://localhost:5672",
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

// jitter returns d ± 25% to spread reconnect attempts across a fleet and avoid
// a thundering-herd storm on the broker after a blip.
func jitter(d time.Duration) time.Duration {
	delta := time.Duration(rand.Int63n(int64(d) / 2)) // 0 to 50% of d
	if rand.Intn(2) == 0 {
		return d - delta/2 // subtract up to 25%
	}
	return d + delta/2 // add up to 25%
}

// connect dials the broker, retrying on failure until success or until the user
// calls Close. ready is the channel the caller owns and expects connect() to close
// once a live connection is stored — passing it explicitly avoids any risk of
// closing a stale or already-closed channel.
//
// Issue #7: the ticker is created only after the first failure so the very first
// dial attempt is immediate.
func (c *Connection) connect(uri string, ready chan struct{}) error {
	var attempts int

	for {
		select {
		case <-c.done:
			return ErrConnectionClosedByUser
		default:
		}

		conn, err := amqp.DialConfig(uri, amqp.Config{
				Heartbeat: c.options.heartbeat,
				Dial:      amqp.DefaultDial(c.options.dialTimeout),
			})
		if err == nil {
			// Fix C8: check done before storing conn and unblocking waiters.
			// The conn is dialled but not yet visible to anyone, so closing it
			// here races nothing and eliminates the window where waiters could
			// grab channels from a conn we are about to tear down.
			select {
			case <-c.done:
				conn.Close()
				return ErrConnectionClosedByUser
			default:
			}

			// Issue #16: notify before closing ready so observers see the notification
			// before getNewChannel unblocks.
			c.state.Store(&connState{conn: conn, ready: ready})
			notifyConnectionEstablished(c.options.notificationChannel)
			close(ready)

			return nil
		}

		attempts++
		if c.options.maxReconnectAttempts > 0 && attempts >= c.options.maxReconnectAttempts {
			notifyConnectionFailed(c.options.notificationChannel, err)
			return ErrMaxReconnectAttemptsReached
		}

		notifyConnectionFailed(c.options.notificationChannel, err)

		// Use a per-iteration Timer (not a Ticker) with jitter so each reconnect
		// waits exactly one jittered interval from the previous attempt — this
		// prevents tick queuing (H7) and spreads fleet reconnects (H6).
		t := time.NewTimer(jitter(c.options.reconnectInterval))
		select {
		case <-c.done:
			t.Stop()
			return ErrConnectionClosedByUser
		case <-t.C:
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
		<-st.conn.NotifyClose(make(chan *amqp.Error, 1))

		// Create the new ready before storing so getNewChannel callers immediately
		// block on it rather than spinning on Channel() calls against the dead conn.
		newReady := make(chan struct{})
		c.state.Store(&connState{conn: nil, ready: newReady})

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

	// Single deadline for the entire Close() operation so the total time spent
	// never exceeds closeTimeout regardless of how many steps are involved.
	timer := time.NewTimer(c.options.closeTimeout)
	defer timer.Stop()

	st := c.state.Load()
	var err error
	if st.conn != nil {
		notifyClosingConnection(c.options.notificationChannel)

		// Run conn.Close() with a timeout to avoid blocking forever on a wedged conn.
		done := make(chan error, 1)
		go func() { done <- st.conn.Close() }()

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

	// Wait for reconnectLoop to exit, reusing the same timer so the combined
	// conn.Close() + wg.Wait() time stays within the single closeTimeout budget.
	waitDone := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(waitDone)
	}()

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
			// State not yet populated — wait for the next ready signal
			select {
			case <-c.done:
				return nil, ErrConnectionClosedByUser
			case <-st.ready:
			}
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
			t := time.NewTimer(c.options.channelRetryInterval)
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
