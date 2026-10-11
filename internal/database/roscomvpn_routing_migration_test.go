package database

import (
	"errors"
	"path/filepath"
	"testing"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestMigrateRoscomVPNRoutingSource(t *testing.T) {
	const defaultURL = "https://raw.githubusercontent.com/hydraponique/roscomvpn-routing/main/HAPP/DEFAULT.DEEPLINK"
	const whitelistURL = "https://raw.githubusercontent.com/hydraponique/roscomvpn-routing/main/HAPP/WHITELIST.DEEPLINK"

	cases := []struct {
		name      string
		settings  map[string]string
		wantRules string
		rulesRow  bool
	}{
		{name: "default profile fills empty rules", settings: map[string]string{"subRoutingSource": "default", "subRoutingRules": ""}, wantRules: defaultURL, rulesRow: true},
		{name: "missing rules row is created", settings: map[string]string{"subRoutingSource": "Whitelist"}, wantRules: whitelistURL, rulesRow: true},
		{name: "admin rules are never overwritten", settings: map[string]string{"subRoutingSource": "default", "subRoutingRules": "happ://routing/off"}, wantRules: "happ://routing/off", rulesRow: true},
		{name: "custom source keeps its own rules", settings: map[string]string{"subRoutingSource": "custom", "subRoutingRules": ""}, wantRules: "", rulesRow: true},
		{name: "vanilla install is untouched", settings: map[string]string{}, rulesRow: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
				t.Fatalf("InitDB: %v", err)
			}
			t.Cleanup(func() { _ = CloseDB() })
			if err := db.Where("seeder_name = ?", "RoscomVPNRoutingSourceToRules").Delete(&model.HistoryOfSeeders{}).Error; err != nil {
				t.Fatalf("reset seeder: %v", err)
			}
			for key, value := range tc.settings {
				if err := db.Create(&model.Setting{Key: key, Value: value}).Error; err != nil {
					t.Fatalf("seed %s: %v", key, err)
				}
			}

			if err := migrateRoscomVPNRoutingSource(); err != nil {
				t.Fatalf("migrate: %v", err)
			}

			var rules model.Setting
			err := db.Where("key = ?", "subRoutingRules").First(&rules).Error
			if tc.rulesRow {
				if err != nil {
					t.Fatalf("subRoutingRules row: %v", err)
				}
				if rules.Value != tc.wantRules {
					t.Fatalf("subRoutingRules = %q, want %q", rules.Value, tc.wantRules)
				}
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				t.Fatalf("subRoutingRules row must not be created, got err=%v value=%q", err, rules.Value)
			}
			var leftover int64
			if err := db.Model(&model.Setting{}).Where("key = ?", "subRoutingSource").Count(&leftover).Error; err != nil {
				t.Fatalf("count subRoutingSource: %v", err)
			}
			if leftover != 0 {
				t.Fatal("subRoutingSource must be removed so the setting is not migrated twice")
			}
		})
	}
}
