package cf

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudflare/cloudflare-go/v7"
	"github.com/cloudflare/cloudflare-go/v7/dns"
	"github.com/cloudflare/cloudflare-go/v7/option"
)

// TunnelService menangani pendaftaran DNS custom domain eksternal klien
// yang diarahkan ke Master Tunnel (Wildcard Magic).
//
// Subdomain platform (*.domain) TIDAK perlu panggilan API: rute wildcard
// Master Tunnel meneruskan semuanya ke Traefik lokal.
type TunnelService struct {
	cfClient   *cloudflare.Client
	accountID  string
	zoneID     string
	mainDomain string
}

// NewTunnelService membuat instance TunnelService
func NewTunnelService(apiToken, accountID, zoneID, mainDomain string) (*TunnelService, error) {
	api := cloudflare.NewClient(option.WithAPIToken(apiToken))
	return &TunnelService{
		cfClient:   api,
		accountID:  accountID,
		zoneID:     zoneID,
		mainDomain: mainDomain,
	}, nil
}

// CreateCNAMERecord mendaftarkan DNS CNAME: hostname → <MASTER_TUNNEL_ID>.cfargotunnel.com
// tunnelID adalah UUID Master Tunnel (CLOUDFLARE_TUNNEL_ID).
func (s *TunnelService) CreateCNAMERecord(ctx context.Context, hostname, tunnelID string) (*dns.RecordResponse, error) {
	target := fmt.Sprintf("%s.cfargotunnel.com", tunnelID)
	log.Printf("[cloudflare] membuat CNAME %s → %s", hostname, target)

	resp, err := s.cfClient.DNS.Records.New(ctx, dns.RecordNewParams{
		ZoneID: cloudflare.F(s.zoneID),
		Body: dns.RecordNewParamsBody{
			Type:    cloudflare.F(dns.RecordNewParamsBodyTypeCNAME),
			Name:    cloudflare.F(hostname),
			Content: cloudflare.F(target),
			Proxied: cloudflare.F(true),     // wajib proxied untuk tunnel
			TTL:     cloudflare.F(dns.TTL1), // 1 = automatic
		},
	})
	if err != nil {
		return nil, fmt.Errorf("gagal membuat CNAME %s: %w", hostname, err)
	}

	log.Printf("[cloudflare] CNAME %s berhasil dibuat (id=%s)", hostname, resp.ID)
	return resp, nil
}

// DeleteDNSRecord menghapus DNS record berdasarkan record ID
func (s *TunnelService) DeleteDNSRecord(ctx context.Context, recordID string) error {
	_, err := s.cfClient.DNS.Records.Delete(ctx, recordID, dns.RecordDeleteParams{
		ZoneID: cloudflare.F(s.zoneID),
	})
	if err != nil {
		return fmt.Errorf("gagal hapus DNS record %s: %w", recordID, err)
	}
	log.Printf("[cloudflare] DNS record %s dihapus", recordID)
	return nil
}
