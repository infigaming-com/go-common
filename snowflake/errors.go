package snowflake

import "errors"

var (
	// ErrClockRollback is returned when system clock moves backward beyond maxClockDrift.
	ErrClockRollback = errors.New("snowflake: clock rollback exceeds max drift")

	// ErrLeaseExpired is returned when the node lease has expired.
	ErrLeaseExpired = errors.New("snowflake: node lease expired")

	// ErrInvalidNodeID is returned when node ID is out of valid range.
	ErrInvalidNodeID = errors.New("snowflake: node ID must be between 0 and 1023")

	// ErrNoAvailableNode is returned when all 1024 node slots are occupied.
	ErrNoAvailableNode = errors.New("snowflake: no available node ID")

	// ErrLeaseNotHeld is returned when trying to renew/release a lease not held by this holder.
	ErrLeaseNotHeld = errors.New("snowflake: lease not held by this holder")
)
