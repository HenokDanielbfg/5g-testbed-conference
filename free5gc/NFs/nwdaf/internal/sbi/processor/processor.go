package processor

import (
	nwdaf_context "github.com/free5gc/nwdaf/internal/context"
	"github.com/free5gc/nwdaf/pkg/factory"
)

// ProcessorNwdaf defines the interface that the Processor depends on for
// accessing configuration and NWDAF context. It is satisfied by the main
// NWDAF application object.
type ProcessorNwdaf interface {
	Config() *factory.Config
	Context() *nwdaf_context.NWDAFContext
}

// Processor holds all analytics business logic for the NWDAF SBI layer.
// It embeds ProcessorNwdaf so that p.Config() and p.Context() are available
// directly on any method receiver of *Processor.
type Processor struct {
	ProcessorNwdaf
}

// NewProcessor constructs a Processor and returns it. An error is returned if
// the provided nwdaf implementation is nil or otherwise invalid.
func NewProcessor(nwdaf ProcessorNwdaf) (*Processor, error) {
	p := &Processor{
		ProcessorNwdaf: nwdaf,
	}
	return p, nil
}
