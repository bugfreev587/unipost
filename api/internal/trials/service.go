package trials

// Store and StripeGateway are intentionally narrow placeholders. Domain
// operations add only the methods they use as those operations are introduced.
type Store interface{}

type StripeGateway interface{}

type Service struct {
	store  Store
	stripe StripeGateway
}

func NewService(store Store, stripe StripeGateway) *Service {
	return &Service{store: store, stripe: stripe}
}
