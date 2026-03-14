package order

// Orderer handles automated server ordering via the Hetzner Robot API.
// Planned for M5 milestone (gated, requires min score 90 + confirmation).
type Orderer interface {
	Order(serverID int) error
}
