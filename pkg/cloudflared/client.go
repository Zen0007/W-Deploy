package cf

import (
	"fmt"

	"github.com/cloudflare/cloudflare-go/v7"
	"github.com/cloudflare/cloudflare-go/v7/option"
)

type ClientCloudflared struct {
	api       *cloudflare.Client
	accountID string
	zoneID    string
}

// NewClient builds a Cloudflare API client. Returns (nil, nil) when no token
// is configured — callers must handle a nil client as "DNS auto-provisioning
// disabled".
func NewClient(apiToken, accountID, zoneID string) (*ClientCloudflared, error) {
	if apiToken == "" {
		return nil, nil
	}
	if accountID == "" {
		return nil, fmt.Errorf("CLOUDFLARE_ACCOUNT_ID wajib diisi ketika CLOUDFLARE_API_TOKEN diset")
	}
	return &ClientCloudflared{
		api:       cloudflare.NewClient(option.WithAPIToken(apiToken)),
		accountID: accountID,
		zoneID:    zoneID,
	}, nil
}

// Enabled reports whether the client is usable.
func (c *ClientCloudflared) Enabled() bool { return c != nil && c.api != nil }

func (c *ClientCloudflared) AccountID() string {
	return c.accountID
}

func (c *ClientCloudflared) ZoneID() string {
	return c.zoneID
}
