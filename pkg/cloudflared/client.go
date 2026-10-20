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

func NewClient(apiToken, accountID, zoneID string) (*ClientCloudflared, error) {
	if apiToken == "" || accountID == "" {
		return nil, fmt.Errorf("CLOUDFLARE_API_TOKEN dan CLOUDFLARE_ACCOUNT_ID wajib diisi")
	}

	api := cloudflare.NewClient(
		option.WithAPIToken(apiToken),
	)
	return &ClientCloudflared{
		api:       api,
		accountID: accountID,
		zoneID:    zoneID,
	}, nil
}

func (c *ClientCloudflared) AccountID() string {
	return c.accountID
}

func (c *ClientCloudflared) ZoneID() string {
	return c.zoneID
}
