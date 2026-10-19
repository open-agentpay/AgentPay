package tests

import (
	"context"
	"testing"
	"time"

	"github.com/open-agentpay/agentpay-gateway-go/internal/wallet"
)

// TestMultiTenantWalletIsolation verifies that virtual wallets, balance holds,
// and ledger entries are completely isolated across tenants with zero crosstalk (T3.5).
func TestMultiTenantWalletIsolation(t *testing.T) {
	ctx := context.Background()
	svc := wallet.NewMemoryService()

	// 1. Setup Tenant Alpha
	walAlpha := &wallet.Wallet{
		WalletID:       "wal_alpha_001",
		TenantID:       "ten_alpha",
		Asset:          "USDC",
		Status:         wallet.StatusActive,
		AvailableCache: 50_000_000, // 50 USDC
		HeldCache:      0,
		Version:        1,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	svc.RegisterWallet(walAlpha)
	svc.RegisterAPIKey("apk_live_alpha_123", "ten_alpha", "prj_alpha")

	// 2. Setup Tenant Beta
	walBeta := &wallet.Wallet{
		WalletID:       "wal_beta_002",
		TenantID:       "ten_beta",
		Asset:          "USDC",
		Status:         wallet.StatusActive,
		AvailableCache: 10_000_000, // 10 USDC
		HeldCache:      0,
		Version:        1,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	svc.RegisterWallet(walBeta)
	svc.RegisterAPIKey("apk_live_beta_456", "ten_beta", "prj_beta")

	// 3. Verify API Key resolves to strict respective tenant wallet
	accAlpha, err := svc.GetWalletByAPIKey(ctx, "apk_live_alpha_123")
	if err != nil || accAlpha.TenantID != "ten_alpha" || accAlpha.WalletID != "wal_alpha_001" {
		t.Fatalf("expected Alpha wallet resolution, got %v, err=%v", accAlpha, err)
	}

	accBeta, err := svc.GetWalletByAPIKey(ctx, "apk_live_beta_456")
	if err != nil || accBeta.TenantID != "ten_beta" || accBeta.WalletID != "wal_beta_002" {
		t.Fatalf("expected Beta wallet resolution, got %v, err=%v", accBeta, err)
	}

	// 4. Perform Financial Hold & Capture on Tenant Alpha
	holdID, err := svc.HoldFunds(ctx, walAlpha.WalletID, 5_000_000, "pin_alpha_tx1")
	if err != nil {
		t.Fatalf("failed to hold funds on Alpha: %v", err)
	}
	if err := svc.CaptureFunds(ctx, holdID); err != nil {
		t.Fatalf("failed to capture funds on Alpha: %v", err)
	}

	// 5. Verify Tenant Beta's balance is 100% unaffected
	wBetaAfter, err := svc.GetWalletByTenant(ctx, "ten_beta")
	if err != nil {
		t.Fatalf("failed to get Beta wallet: %v", err)
	}
	if wBetaAfter.AvailableCache != 10_000_000 || wBetaAfter.HeldCache != 0 {
		t.Fatalf("isolation breach: Tenant Beta balance modified by Tenant Alpha operation! %v", wBetaAfter)
	}

	// 6. Verify Ledger Entries are strictly partitioned
	ledgerAlpha, err := svc.GetLedgerEntries(ctx, walAlpha.WalletID)
	if err != nil {
		t.Fatalf("failed to get Alpha ledger: %v", err)
	}
	if len(ledgerAlpha) == 0 {
		t.Fatalf("expected Alpha ledger entries")
	}

	ledgerBeta, err := svc.GetLedgerEntries(ctx, walBeta.WalletID)
	if err != nil {
		t.Fatalf("failed to get Beta ledger: %v", err)
	}
	if len(ledgerBeta) != 0 {
		t.Fatalf("isolation breach: Tenant Beta sees ledger entries from Tenant Alpha! len=%d", len(ledgerBeta))
	}

	// 7. Verify ReconcileBalance isolates tenants
	reportAlpha, err := svc.ReconcileBalance(ctx, walAlpha.WalletID)
	if err != nil || !reportAlpha.IsBalanced {
		t.Fatalf("Alpha reconciliation failed: %v", reportAlpha)
	}

	reportBeta, err := svc.ReconcileBalance(ctx, walBeta.WalletID)
	if err != nil || !reportBeta.IsBalanced {
		t.Fatalf("Beta reconciliation failed: %v", reportBeta)
	}
}
