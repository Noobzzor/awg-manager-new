package forwarding

import "context"

type Reconciler struct {
	SetIPv4Forwarding func(context.Context, bool) error
	ApplyNFT          func(context.Context, string) error
}
