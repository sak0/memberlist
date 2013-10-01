package memberlist

import (
	"sort"
	"sync"
)

// TransmitLimitedQueue is used to queue messages to broadcast but
// limits the number of transmits per message, and also prioritises
// messages with lower transmit counts (hence new messages).
type TransmitLimitedQueue struct {
	sync.Mutex
	bcQueue        limitedBroadcasts
	NumNodes       func() int
	RetransmitMult int
}

// limitedBroadcast is used with the TransmitLimitedQueue
type limitedBroadcast struct {
	transmits int // Number of transmissions
	b         Broadcast
}
type limitedBroadcasts []*limitedBroadcast

// Broadcast is something that can be put into the queue
type Broadcast interface {
	// Invalidates checks if enqueuing the current broadcast
	// invalidates a previous broadcast
	Invalidates(b Broadcast) bool

	// Returns a byte form of the message
	Message() []byte
}

// QueueBroadcast is used to enqueue a broadcast
func (q *TransmitLimitedQueue) QueueBroadcast(b Broadcast) {
	q.Lock()
	defer q.Unlock()

	// Check if this message invalidates another
	n := len(q.bcQueue)
	for i := 0; i < n; i++ {
		if b.Invalidates(q.bcQueue[i].b) {
			copy(q.bcQueue[i:], q.bcQueue[i+1:])
			q.bcQueue[n-1] = nil
			q.bcQueue = q.bcQueue[:n-1]
			n--
		}
	}

	// Append to the queue
	q.bcQueue = append(q.bcQueue, &limitedBroadcast{0, b})
}

// GetBroadcasts is used to get a number of broadcasts, up to a byte limit
// and applying a per-message overhead as provided.
func (q *TransmitLimitedQueue) GetBroadcasts(overhead, limit int) [][]byte {
	q.Lock()
	defer q.Unlock()

	transmitLimit := retransmitLimit(q.RetransmitMult, q.NumNodes())
	bytesUsed := 0
	var toSend [][]byte

	for i := len(q.bcQueue) - 1; i >= 0; i-- {
		// Check if this is within our limits
		b := q.bcQueue[i]
		msg := b.b.Message()
		if bytesUsed+overhead+len(msg) > limit {
			continue
		}

		// Add to slice to send
		bytesUsed += overhead + len(msg)
		toSend = append(toSend, msg)

		// Check if we should stop transmission
		b.transmits++
		if b.transmits >= transmitLimit {
			n := len(q.bcQueue)
			q.bcQueue[i], q.bcQueue[n-1] = q.bcQueue[n-1], nil
			q.bcQueue = q.bcQueue[:n-1]
		}
	}

	// If we are sending anything, we need to re-sort to deal
	// with adjusted transmit counts
	if len(toSend) > 0 {
		q.bcQueue.Sort()
	}
	return toSend
}

// NumQueued returns the number of queued messages
func (q *TransmitLimitedQueue) NumQueued() int {
	q.Lock()
	defer q.Unlock()
	return len(q.bcQueue)
}

// Reset clears all the queued messages
func (q *TransmitLimitedQueue) Reset() {
	q.Lock()
	defer q.Unlock()
	q.bcQueue = nil
}

func (b limitedBroadcasts) Len() int {
	return len(b)
}

func (b limitedBroadcasts) Less(i, j int) bool {
	return b[i].transmits < b[j].transmits
}

func (b limitedBroadcasts) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b limitedBroadcasts) Sort() {
	sort.Sort(sort.Reverse(b))
}
