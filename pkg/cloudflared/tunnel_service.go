package cf

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudflare/cloudflare-go/v7"
	"github.com/cloudflare/cloudflare-go/v7/dns"
	"github.com/cloudflare/cloudflare-go/v7/option"
	"github.com/cloudflare/cloudflare-go/v7/zero_trust"
)

// TunnelService menangani pembuatan Named Tunnel & DNS CNAME
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

// CreateTunnel membuat Named Tunnel baru via POST /accounts/{account_id}/cfd_tunnel
func (s *TunnelService) CreateTunnel(ctx context.Context, name string) (*zero_trust.CloudflareTunnel, error) {
	log.Printf("[cloudflare] membuat tunnel '%s'...", name)

	resp, err := s.cfClient.ZeroTrust.Tunnels.Cloudflared.New(ctx, zero_trust.TunnelCloudflaredNewParams{
		AccountID: cloudflare.F(s.accountID),
		Name:      cloudflare.F(name),
		ConfigSrc: cloudflare.F(zero_trust.TunnelCloudflaredNewParamsConfigSrcCloudflare), // remote-managed (via dashboard)
	})
	if err != nil {
		return nil, fmt.Errorf("gagal membuat tunnel '%s': %w", name, err)
	}

	// TIPS: resp.TunnelSecret hanya akan muncul sekali di respons ini.
	// Jika aplikasi Anda butuh menjalankan cloudflared, pastikan secret ini
	// disimpan (misalnya ke Vault/Database) atau diconvert menjadi Tunnel Token.

	log.Printf("[cloudflare] tunnel '%s' berhasil dibuat (id=%s)", name, resp.ID)
	return resp, nil
}

// GetTunnelToken mengambil token tunnel untuk dijalankan oleh cloudflared
func (s *TunnelService) GetTunnelToken(ctx context.Context, tunnelID string) (string, error) {
	// Di v7, panggil langsung Tunnels.Token(ctx, tunnelID, params)
	resp, err := s.cfClient.ZeroTrust.Tunnels.Cloudflared.Token.Get(ctx, tunnelID, zero_trust.TunnelCloudflaredTokenGetParams{
		AccountID: cloudflare.F(s.accountID),
	})
	if err != nil {
		return "", fmt.Errorf("gagal mengambil token untuk tunnel '%s': %w", tunnelID, err)
	}

	return *resp, nil // Di v7, kembaliannya berupa string primitif langsung
}

// DeleteTunnel menghapus tunnel beserta koneksinya
func (s *TunnelService) DeleteTunnel(ctx context.Context, tunnelID string) error {
	_, err := s.cfClient.ZeroTrust.Tunnels.Cloudflared.Delete(ctx, tunnelID, zero_trust.TunnelCloudflaredDeleteParams{
		AccountID: cloudflare.F(s.accountID),
	})
	if err != nil {
		return fmt.Errorf("gagal hapus tunnel %s: %w", tunnelID, err)
	}
	log.Printf("[cloudflare] tunnel %s dihapus", tunnelID)
	return nil
}

// CreateCNAMERecord mendaftarkan DNS CNAME: subdomain → <TUNNEL_ID>.cfargotunnel.com
// Target format: <UUID>.cfargotunnel.com
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

// ProvisionApp membuat tunnel + CNAME untuk aplikasi baru dalam satu alur
func (s *TunnelService) ProvisionApp(ctx context.Context, appSlug string) (*ProvisionResult, error) {
	tunnelName := fmt.Sprintf("paas-%s", appSlug)
	hostname := fmt.Sprintf("%s.%s", appSlug, s.mainDomain)

	// 1. Buat tunnel
	tunnel, err := s.CreateTunnel(ctx, tunnelName)
	if err != nil {
		return nil, err
	}

	// 2. Buat CNAME
	dnsRecord, err := s.CreateCNAMERecord(ctx, hostname, tunnel.ID)
	if err != nil {
		// Rollback: hapus tunnel jika CNAME gagal
		_ = s.DeleteTunnel(ctx, tunnel.ID)
		return nil, fmt.Errorf("CNAME gagal, tunnel di-rollback: %w", err)
	}

	// 3. Ambil token
	token, err := s.GetTunnelToken(ctx, tunnel.ID)
	if err != nil {
		log.Printf("[cloudflare] warning: gagal ambil token tunnel %s: %v", tunnel.ID, err)
	}

	return &ProvisionResult{
		TunnelID:    tunnel.ID,
		TunnelName:  tunnelName,
		Hostname:    hostname,
		DNSRecordID: dnsRecord.ID,
		Token:       token,
	}, nil
}

// ProvisionResult hasil provisioning
type ProvisionResult struct {
	TunnelID    string
	TunnelName  string
	Hostname    string
	DNSRecordID string
	Token       string
}
